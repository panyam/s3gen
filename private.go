package s3gen

import "path/filepath"

type defaultResourceLoader struct {
}

func (m *defaultResourceLoader) IsParametric(res *Resource) bool {
	we := res.WithoutExt(true)
	base := filepath.Base(we)
	return base[0] == '[' && base[len(base)-1] == ']'
}

func (m *defaultResourceLoader) LoadPage(res *Resource, page *Page) error {
	return nil
}

func (m *defaultResourceLoader) LoadParamValues(res *Resource) error {
	return nil
}

func (m *defaultResourceLoader) SetupPageView(res *Resource, page *Page) (err error) {
	return nil
}
