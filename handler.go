package s3gen

import (
	"errors"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"
)

// Loads a resource of diferent types from storage
type ResourceHandler interface {
	// Loads resource data from the appropriate input path
	LoadResource(r *Resource) error

	// Generates all target/child resources for a given resources.
	// Note that before this method is called, LoadResource and LoadParamValues
	// must have be called otherwise this wont work well on resources which are parametric
	GenerateTargets(r *Resource, deps map[string]*Resource) error

	// Loads the parameter values for a resource.  Parametric resources are an important concept.
	// Parametric resources are a way to ensure that a given reosurce (eg a page) can take several instances.
	// For example the content page `<content_root>/tags/[tag].md` can resultin multiple files of the form
	// `<output_folder>/tags/tag1/index.html`, `<output_folder>/tags/tag2/index.html` and so on.  This is
	// evaluated by rendering the the source file (`<content_root>/tags/[tag].md`)  in the "nameless" mode
	// where the template would call the AddParam method once for each new child resources (eg tag1, tag2...)
	// until all param names are resolved.  And then once for each parameter the template is rendered again
	// so that the correspond output page is generated.  This may seem like a lot but since static sites need
	// all combinations generated upfront, this is fine as long as the number of variations are small.
	// This method performs the "nameless" mode rendering to gather all parameter values a parametric page can take.
	LoadParamValues(r *Resource) error

	// Renders just the content section within the resource so that it can be embedded in the "larger" layout page.
	RenderContent(res *Resource, w io.Writer) error

	// Once the content (ie the main body) of the source is rendered,
	// it needs to be wrapped up in a larger view so it finally looks
	// like a rendered page.  This method should be called on the
	// Destination resource to perform the final wrapping.
	RenderResource(res *Resource, content any, w io.Writer) error
}

type defaultResourceHandler struct {
}

/*
func (m *defaultResourceHandler) IsParametric(res *Resource) bool {
	we := res.WithoutExt(true)
	base := filepath.Base(we)
	return base[0] == '[' && base[len(base)-1] == ']'
}
*/

func (m *defaultResourceHandler) LoadParamValues(res *Resource) error {
	return nil
}

// Generates all target resources for a given resources.
// Note that before this method is called, LoadResource and LoadParamValues
// must have be called otherwise this wont work well on resources which are parametric
func (m *defaultResourceHandler) GenerateTargets(r *Resource, deps map[string]*Resource) (err error) {
	s := r.Site
	s.RemoveEdgesFrom(r.FullPath)
	respath, found := strings.CutPrefix(r.FullPath, s.ContentRoot)
	if !found {
		log.Println("Respath not found: ", r.FullPath, s.ContentRoot)
		return nil
	}

	if r.IsParametric {
		ext := filepath.Ext(respath)

		rem := respath[:len(respath)-len(ext)]
		dirname := filepath.Dir(rem)

		// TODO - also see if there is a .<lang> prefix on rem after
		// ext has been removed can use that for language sites
		for _, paramName := range r.ParamValues {
			destpath := filepath.Join(s.OutputDir, dirname, paramName, "index.html")
			destres := s.GetResource(destpath)
			destres.Source = r
			destres.Page = r.Page
			destres.frontMatter = r.frontMatter
			destres.ParamName = paramName
			if s.AddEdge(r.FullPath, destres.FullPath) {
				if deps != nil {
					deps[destres.FullPath] = destres
				}
			} else {
				log.Printf("Found cycle with edge from %s -> %s", r.FullPath, destres.FullPath)
			}
		}
	} else {
		// we have a basic so generate it
		destpath := ""
		if r.Info().IsDir() {
			// Then this will be served with dest/index.html
			destpath = filepath.Join(s.OutputDir, respath)
		} else if r.IsIndex {
			destpath = filepath.Join(s.OutputDir, filepath.Dir(respath), "index.html")
		} else if r.NeedsIndex {
			// res is not a dir - eg it something like xyz.ext
			// depending on ext - if the ext is for a page file
			// then generate OutDir/xyz/index.html
			// otherwise OutDir/xyz.ext
			ext := filepath.Ext(respath)

			rem := respath[:len(respath)-len(ext)]

			// TODO - also see if there is a .<lang> prefix on rem after ext has been removed
			// can use that for language sites
			destpath = filepath.Join(s.OutputDir, rem, "index.html")
		} else {
			// basic static file - so copy as is
			destpath = filepath.Join(s.OutputDir, respath)
		}
		destres := s.GetResource(destpath)
		destres.Source = r
		destres.Page = r.Page
		destres.frontMatter = r.frontMatter
		if s.AddEdge(r.FullPath, destres.FullPath) {
			if deps != nil {
				deps[destres.FullPath] = destres
			}
		} else {
			log.Printf("Found cycle with edge from %s -> %s", r.FullPath, destres.FullPath)
		}
	}
	return
}

func (m *defaultResourceHandler) getResourceTemplate(res *Resource) (engine string, template PageTemplate, err error) {
	frontMatter := res.FrontMatter().Data

	// we want to support different kinds of templating engines, renderes etc
	// which rendering engine to use
	engine = "views"
	if frontMatter["engine"] != nil {
		engine = frontMatter["engine"].(string)
	}

	// which page template to use

	if res.Site.GetTemplate != nil {
		res.Site.GetTemplate(res, &template)
	}

	// Get any missing template details from the default
	if template.Name == "" {
		template.Name = res.Site.DefaultPageTemplate.Name
	}
	if template.Params == nil {
		template.Params = res.Site.DefaultPageTemplate.Params
	}

	// now see if we can override them what is on the page
	if frontMatter["template"] != nil {
		template.Name = frontMatter["template"].(string)
	}
	if frontMatter["templateParams"] != nil {
		template.Params = frontMatter["templateParams"].(map[any]any)
	}
	return
}

func (m *defaultResourceHandler) RenderResource(outres *Resource, content any, writer io.Writer) error {
	if outres.Source == nil {
		log.Println("Resource does not have a source: ", outres.FullPath)
		return errors.New("resource does not have a source")
	}

	// we want to support different kinds of templating engines, renderes etc
	// which rendering engine to use
	_, template, err := m.getResourceTemplate(outres.Source)
	if err != nil {
		return err
	}
	// Assume html template for now
	params := map[any]any{
		"Res":         outres,
		"Site":        outres.Site,
		"FrontMatter": outres.FrontMatter().Data,
		"Content":     content,
	}
	if template.Params != nil {
		for k, v := range template.Params {
			params[k] = v
		}
	}

	// TODO - check if this should always pick a html template?
	err = outres.Site.HtmlTemplate(false).ExecuteTemplate(writer, template.Name, params)
	if err != nil {
		log.Println("Error rendering template: ", outres.FullPath, template, err)
		_, err = writer.Write([]byte(fmt.Sprintf("Template error: %s", err.Error())))
	}
	return err
}
