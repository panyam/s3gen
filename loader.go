package s3gen

import (
	"path/filepath"
)

// Loads a resource of diferent types from storage
type ResourceLoader interface {
	// Loads resource data from the appropriate input path
	LoadResource(r *Resource) error

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
	SetupPageView(res *Resource, page *Page) (err error)
}

type DefaultResourceLoader struct {
}

func (m *DefaultResourceLoader) IsParametric(res *Resource) bool {
	we := res.WithoutExt(true)
	base := filepath.Base(we)
	return base[0] == '[' && base[len(base)-1] == ']'
}

func (m *DefaultResourceLoader) LoadParamValues(res *Resource) error {
	return nil
}

func (m *DefaultResourceLoader) SetupPageView(res *Resource, page *Page) (err error) {
	return nil
}
