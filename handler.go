package s3gen

import (
	"io"
	"log"
	"path/filepath"
	"strings"
)

// Loads a resource of diferent types from storage
type ResourceHandler interface {
	// Loads resource data from the appropriate input path
	LoadResource(r *Resource) error

	// Generates all target resources for a given resources.
	// Note that before this method is called, LoadResource and LoadParamValues
	// must have be called otherwise this wont work well on resources which are parametric
	GenerateTargets(r *Resource, deps map[string]*Resource) error

	// Loads the parameter values for a resource
	// This is seperate from the resource as this is called only for
	// a paraametric page.  Typically parametric pages will need to know
	// about everything else in the site so the site and its (leaf) resource
	// has to be loaded before this is called.  Hence it is seperated from
	// the normal (leaf) load of a resource.  If the load is successful
	// thenthe r.ParamValues is set to all the parametrics this page can take
	// otherwise an error is returned
	LoadParamValues(r *Resource) error

	// Sets up the view for a page
	GetRenderer(res *Resource) func(*Resource, io.Writer) error
}

type defaultResourceHandler struct {
}

func (m *defaultResourceHandler) IsParametric(res *Resource) bool {
	we := res.WithoutExt(true)
	base := filepath.Base(we)
	return base[0] == '[' && base[len(base)-1] == ']'
}

func (m *defaultResourceHandler) LoadPage(res *Resource, page *Page) error {
	return nil
}

func (m *defaultResourceHandler) LoadParamValues(res *Resource) error {
	return nil
}

func (m *defaultResourceHandler) SetupPageView(res *Resource, page *Page) (err error) {
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
		// we should *never* come here? because the renderer will ensure
		// all "child" resources will be created for parameters
		log.Fatal("Parametric pages wont have a destination path: ", r.FullPath)
		ext := filepath.Ext(respath)

		rem := respath[:len(respath)-len(ext)]
		dirname := filepath.Dir(rem)

		// TODO - also see if there is a .<lang> prefix on rem after
		// ext has been removed can use that for language sites
		for _, paramName := range r.ParamValues {
			destpath := filepath.Join(s.OutputDir, dirname, paramName, "index.html")
			destres := s.GetResource(destpath)
			destres.Source = r
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
