package s3gen

import (
	"bytes"
	"fmt"
	"html/template"
	htmpl "html/template"
	"io"
	"log"
	"log/slog"
	"reflect"
	ttmpl "text/template"

	"github.com/panyam/s3gen/funcs"
	"github.com/panyam/s3gen/views"
)

type TemplateStore struct {
	// A map of template functions available for both text and html templates
	CommonFuncMap htmpl.FuncMap

	// Global templates dirs
	// A list of GLOBs that will point to several html templates our generator will parse and use
	HtmlTemplates []string

	htmlTemplateClone *htmpl.Template
	htmlTemplate      *htmpl.Template
	HtmlFuncMap       htmpl.FuncMap

	// A list of GLOBs that will point to several text templates our generator will parse and use
	TextTemplates     []string
	textTemplateClone *ttmpl.Template
	textTemplate      *ttmpl.Template
	TextFuncMap       ttmpl.FuncMap
}

// Returns the parsed Text templates used for rendering a given template content
// If the clone parameter is true then a clone of the template is returned.  This is useful
// if we ever need to reuse the template while it is in the middle of an execution.
func (s *TemplateStore) TextTemplate(clone bool) *ttmpl.Template {
	if s.textTemplate == nil {
		s.textTemplate = ttmpl.New("SiteTextTemplate").
			Funcs(s.DefaultFuncMap()).
			Funcs(funcs.DefaultFuncMap())
		if s.CommonFuncMap != nil {
			s.textTemplate = s.textTemplate.Funcs(s.CommonFuncMap)
		}
		if s.TextFuncMap != nil {
			s.textTemplate = s.textTemplate.Funcs(s.TextFuncMap)
		}
		for _, templatesDir := range s.TextTemplates {
			t, err := s.textTemplate.ParseGlob(templatesDir)
			if err != nil {
				log.Println("Error parsing templates glob: ", templatesDir)
			} else {
				s.textTemplate = t
				log.Println("Loaded Text Templates")
			}
		}
		var err error
		s.textTemplateClone, err = s.textTemplate.Clone()
		if err != nil {
			log.Println("TextTemplate Clone error: ", err)
		}
	}
	out := s.textTemplate
	if clone {
		var err error
		out, err = s.textTemplateClone.Clone()
		if err != nil {
			log.Println("Text Template Clone Error: ", err)
		}
	}
	return out
}

// Returns the parsed HTML templates used for rendering a given template content
// If the clone parameter is true then a clone of the template is returned.  This is useful
// if we ever need to reuse the template while it is in the middle of an execution.
func (s *TemplateStore) HtmlTemplate(clone bool) *htmpl.Template {
	if s.htmlTemplate == nil {
		s.htmlTemplate = htmpl.New("SiteHtmlTemplate").
			Funcs(s.DefaultFuncMap()).
			Funcs(funcs.DefaultFuncMap())
		if s.CommonFuncMap != nil {
			s.htmlTemplate = s.htmlTemplate.Funcs(s.CommonFuncMap)
		}
		if s.HtmlFuncMap != nil {
			s.htmlTemplate = s.htmlTemplate.Funcs(s.HtmlFuncMap)
		}

		for _, templatesDir := range s.HtmlTemplates {
			slog.Info("Loaded HTML Template: ", "templatesDir", templatesDir)
			t, err := s.htmlTemplate.ParseGlob(templatesDir)
			if err != nil {
				slog.Error("Error parsing templates glob: ", "templatesDir", templatesDir, "error", err)
			} else {
				s.htmlTemplate = t
				slog.Info("Loaded HTML Template: ", "templatesDir", templatesDir)
			}
		}
		var err error
		s.htmlTemplateClone, err = s.htmlTemplate.Clone()
		if err != nil {
			log.Println("HtmlTemplate Clone error: ", err)
		}
	}
	out := s.htmlTemplate
	if clone {
		var err error
		out, err = s.htmlTemplateClone.Clone()
		if err != nil {
			log.Println("Html Template Clone Error: ", err)
		}
	}
	return out
}

// Returns the default function map to be used in the html templates.
func (s *TemplateStore) DefaultFuncMap() htmpl.FuncMap {
	return htmpl.FuncMap{
		"RenderView": func(view views.ViewRenderer) (out template.HTML, err error) {
			if view == nil {
				return "", fmt.Errorf("view is nil")
			}
			output := bytes.NewBufferString("")
			err = s.RenderView(output, view, "")
			return template.HTML(output.String()), err
		},
		"HtmlTemplate": func(templateName string, params any) (out template.HTML, err error) {
			writer := bytes.NewBufferString("")
			err = s.HtmlTemplate(false).ExecuteTemplate(writer, templateName, params)
			out = template.HTML(writer.String())
			return
		},
		"TextTemplate": func(templateName string, params any) (out string, err error) {
			writer := bytes.NewBufferString("")
			err = s.TextTemplate(false).ExecuteTemplate(writer, templateName, params)
			out = writer.String()
			return
		},
	}
}

// Renders a html template
func (s *TemplateStore) RenderHtml(templateName string, params map[string]any) (template.HTML, error) {
	out := bytes.NewBufferString("")
	err := s.HtmlTemplate(false).ExecuteTemplate(out, templateName, params)
	return template.HTML(out.String()), err
}

// Renders a text template
func (s *TemplateStore) RenderText(templateName string, params map[string]any) (template.HTML, error) {
	out := bytes.NewBufferString("")
	err := s.TextTemplate(false).ExecuteTemplate(out, templateName, params)
	return template.HTML(out.String()), err
}

// Site extension to render a view
func (s *TemplateStore) RenderView(writer io.Writer, v views.ViewRenderer, templateName string) error {
	if templateName == "" {
		templateName = v.TemplateName()
	}
	if templateName == "" {
		templateName = s.defaultViewTemplate(v)
		if s.HtmlTemplate(false).Lookup(templateName) == nil {
			templateName = templateName + ".html"
		}
		if s.HtmlTemplate(false).Lookup(templateName) != nil {
			err := s.HtmlTemplate(false).ExecuteTemplate(writer, templateName, v)
			if err != nil {
				log.Println("Error with e.Name().html, Error: ", templateName, err)
				_, err = writer.Write([]byte(fmt.Sprintf("Template error: %s", err.Error())))
			}
			return err
		}
		templateName = ""
	}
	if templateName != "" {
		return s.HtmlTemplate(false).ExecuteTemplate(writer, templateName, v)
	}
	// How do you use the View's renderer func here?
	return v.RenderResponse(writer)
}

func (s *TemplateStore) defaultViewTemplate(v views.ViewRenderer) string {
	t := reflect.TypeOf(v)
	e := t.Elem()
	return e.Name()
}
