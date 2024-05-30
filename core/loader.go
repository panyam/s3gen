package core

import (
	"path/filepath"
)

// Loads a resource of diferent types from storage
type ResourceLoader interface {
	// Loads resource data from the appropriate input path
	LoadResource(s *Site, r *Resource) error

	// Sets up the view for a page
	SetupPageView(res *Resource, page *Page) (err error)

	// Called to handle a resource -
	// This can generate more resources

	// Loads page's details form the frontmatter of a resource
	// LoadPage(res *Resource, page *Page) error
	GeneratePage(res *Resource, page *Page, param string)
	// Process(s *Site, inres *Resource, writer io.Writer) error
}

type DefaultResourceLoader struct {
}

func (m *DefaultResourceLoader) IsParametric(s *Site, res *Resource) bool {
	we := res.WithoutExt(true)
	base := filepath.Base(we)
	return base[0] == '[' && base[len(base)-1] == ']'
}

func (m *DefaultResourceLoader) GetParamValues(s *Site, res *Resource) []string {
	return nil
}

func (m *DefaultResourceLoader) GeneratePage(res *Resource, page *Page, param string) {
}

func (m *DefaultResourceLoader) SetupPageView(res *Resource, page *Page) (err error) {
	return nil
}
