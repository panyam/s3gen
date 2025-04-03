package s3gen

import (
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"strings"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
	"go.abhg.dev/goldmark/anchor"
)

// BaseTemplates are data used to render a page.  Typically this needs the name of the template
// being rendered and the (possibly nested) parameters need by that specific template.
type BaseTemplate struct {
	// Name of the Template file that is to be used as the root
	Name string

	// Name of the template within the template file for the entry point (given a
	// template file may contain multiple templates)
	Entry string

	Params map[any]any
}

type BaseResourceRenderer struct {
}

func (m *BaseResourceRenderer) Render(outres *Resource, w io.Writer) error {
	inres := outres.Source

	if outres.Source == nil {
		log.Println("Resource does not have a source: ", outres.FullPath)
		return errors.New("resource does not have a source")
	}

	template, err := m.getResourceTemplate(inres)
	if err != nil {
		return err
	}
	// Assume html template for now
	params := map[any]any{
		"Res":         outres,
		"Site":        inres.Site,
		"FrontMatter": inres.FrontMatter().Data,
		"Content":     inres.Document.Root,
	}
	if template.Params != nil {
		maps.Copy(params, template.Params)
	}

	// TODO - check if this should always pick a html template?
	tmpl, err := outres.Site.Templates.Loader.Load(template.Name, "")
	if err != nil {
		panic(err)
		// return err
	}
	err = outres.Site.Templates.RenderHtmlTemplate(w, tmpl[0], template.Entry, params, nil)
	if err != nil {
		log.Println("Error rendering template: ", outres.FullPath, template, err)
		log.Println("Contents: ", string(tmpl[0].RawSource))
		_, err = w.Write(fmt.Appendf(nil, "Template error: %s", err.Error()))
	}
	return err
}

func (m *BaseResourceRenderer) getResourceTemplate(res *Resource) (template BaseTemplate, err error) {
	frontMatter := res.FrontMatter().Data

	// Start with the default
	template = res.Site.DefaultBaseTemplate

	// which page template to use
	if res.Site.GetTemplate != nil {
		res.Site.GetTemplate(res, &template)
	}

	// now see if we can override them what is on the page
	if frontMatter["template"] != nil && frontMatter["template"] != "" {
		templateAndEntry := strings.Split(frontMatter["template"].(string), "/")
		template.Name = templateAndEntry[0]
		if len(templateAndEntry) > 1 {
			template.Entry = templateAndEntry[1]
		}
	}
	if frontMatter["templateParams"] != nil {
		template.Params = frontMatter["templateParams"].(map[any]any)
	}
	return
}

type MDResourceRenderer struct {
	BaseResourceRenderer
	md goldmark.Markdown
}

func NewMDResourceRenderer() (out *MDResourceRenderer) {
	out = &MDResourceRenderer{}
	out.md = goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Typographer,
			highlighting.NewHighlighting(
				highlighting.WithStyle("monokai"),
				highlighting.WithFormatOptions(
				// chromahtml.WithLineNumbers(true),
				),
			),
			&anchor.Extender{},
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			html.WithUnsafe(),
		),
	)
	return out
}
