package s3gen

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"path/filepath"

	"github.com/morrisxyang/xreflect"
	gut "github.com/panyam/goutils/utils"
)

func SetNestedProp(obj any, value any, fieldpath string) error {
	return xreflect.SetEmbedField(obj, fieldpath, value)
}

// Site extension to render a view
func (s *Site) RenderView(writer io.Writer, v View, templateName string) error {
	if templateName == "" {
		templateName = v.TemplateName()
	}
	if templateName != "" {
		return s.HtmlTemplate().ExecuteTemplate(writer, templateName, v)
	}
	return v.RenderResponse(writer)
}

// Renders a html template
func (s *Site) RenderHtml(templateName string, params map[string]any) (template.HTML, error) {
	out := bytes.NewBufferString("")
	err := s.HtmlTemplate().ExecuteTemplate(out, templateName, params)
	return template.HTML(out.String()), err
}

// Renders a text template
func (s *Site) RenderText(templateName string, params map[string]any) (template.HTML, error) {
	out := bytes.NewBufferString("")
	err := s.TextTemplate().ExecuteTemplate(out, templateName, params)
	return template.HTML(out.String()), err
}

func (s *Site) Json(path string, fieldpath string) (any, error) {
	if path[0] == '/' {
		return nil, fmt.Errorf("Invalid json file: %s.  Cannot start with a /", path)
	}
	fullpath := gut.ExpandUserPath(filepath.Join(s.ContentRoot, path))
	res := s.GetResource(fullpath)
	if res.Ext() != ".json" {
		return nil, fmt.Errorf("Invalid json file: %s, Ext: %s", fullpath, res.Ext())
	}

	data, err := res.ReadAll()
	if err != nil {
		return nil, err
	}
	return gut.JsonDecodeBytes(data)
}
