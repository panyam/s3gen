package core

import (
	htmpl "html/template"
	"io"
	"log"
	"log/slog"
	"path/filepath"
	"strings"
)

type HTMLResourceLoader struct {
	DefaultResourceLoader
	Template *htmpl.Template
}

func NewHTMLResourceLoader(templatesDir string) *HTMLResourceLoader {
	h := &HTMLResourceLoader{}
	h.Template = htmpl.New("hello")
	if templatesDir != "" {
		// Funcs(CustomFuncMap()).
		t, err := h.Template.ParseGlob(templatesDir)
		if err != nil {
			log.Println("Error loading dir: ", templatesDir, err)
		} else {
			h.Template = t
		}
	}
	return h
}

func (m *HTMLResourceLoader) LoadResource(s *Site, res *Resource) error {
	base := filepath.Base(res.FullPath)
	res.IsIndex = base == "index.htm" || base == "_index.htm" || base == "index.html" || base == "_index.html"
	res.NeedsIndex = strings.HasSuffix(res.FullPath, ".htm") || strings.HasSuffix(res.FullPath, ".html")

	base = filepath.Base(res.WithoutExt(true))
	res.IsParametric = base[0] == '[' && base[len(base)-1] == ']'

	// if we are not parametric - then created the destination page
	if !res.IsParametric {
		res.DestPage = &Page{Site: s}
		res.DestPage.LoadFrom(res)
	} else {
		// what
	}
	return nil
}

// For a given resource - we need the page data to be populated
// and also we need to find the right View for it.   Great thing
// is our views are strongly typed.
//
// In next pages are organized by folders - same here
// Just that we are doing this via the PopulatePage hook.
// The goal of this function is to do 2 things looking at the
// Resource
// 1. Identify the Page properties (like title, slug etc and any others - may be this can come from FrontMatter?)
// 2. More importantly - Return the PageView type that can render
// the resource.
func (m *HTMLResourceLoader) SetupPageView(res *Resource, page *Page) (err error) {
	// log.Println("RelPath, Link: ", relpath, page.Link)
	frontMatter := res.FrontMatter().Data
	location := "BodyView"
	if frontMatter["location"] != nil {
		location = frontMatter["location"].(string)
	}

	/*
		wrapper := ""
		if frontMatter["wrapper"] != nil {
			wrapper = frontMatter["wrapper"].(string)
		}
		if wrapper != "" {
			// then create another view such that its BodyView
			WrapperView{BaseView: BaseView{Template: wrapper}}
		}
	*/

	view := &HTMLView{Res: res, Page: page}
	// log.Println("Before pageName, location: ", pageName, location, page.RootView, mdview)
	// defer log.Println("After pageName, location: ", pageName, location, page.RootView)
	return SetViewProp(page.RootView, view, location)
}

// A view that renders a Markdown
type HTMLView struct {
	BaseView

	// Page we are rendering into
	Page *Page

	// Actual resource to render
	Res *Resource
}

func (v *HTMLView) InitView(site *Site, parentView View) {
	if v.Self == nil {
		v.Self = v
	}
	v.BaseView.InitView(site, parentView)
}

func (v *HTMLView) RenderResponse(writer io.Writer) (err error) {
	res := v.Res
	mdfile, _ := res.Reader()
	mddata, _ := io.ReadAll(mdfile)
	defer mdfile.Close()

	mdTemplate, err := v.Site.HtmlTemplateClone().Parse(string(mddata))
	if err != nil {
		slog.Error("Template Clone Error: ", "error", err)
		return err
	}
	if err != nil {
		mdTemplate, err = mdTemplate.Parse(string(mddata))
		return err
	}

	err = mdTemplate.Execute(writer, v)
	if err != nil {
		log.Println("Error executing HTML: ", res.FullPath, err)
	}
	return err
}
