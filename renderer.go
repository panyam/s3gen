package s3gen

import (
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
)

type BaseResourceRenderer struct {
}

func (m *BaseResourceRenderer) Render(outres *Resource, w io.Writer) error {
	inres := outres.Source

	if outres.Source == nil {
		log.Println("Resource does not have a source: ", outres.FullPath)
		return errors.New("resource does not have a source")
	}

	// we want to support different kinds of templating engines, renderes etc
	// which rendering engine to use
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
		for k, v := range template.Params {
			params[k] = v
		}
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
		_, err = w.Write([]byte(fmt.Sprintf("Template error: %s", err.Error())))
	}
	return err
}

func (m *BaseResourceRenderer) getResourceTemplate(res *Resource) (template PageTemplate, err error) {
	frontMatter := res.FrontMatter().Data

	// Start with the default
	template = res.Site.DefaultPageTemplate

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
