package s3gen

import (
	"bytes"
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

func (m *HTMLResourceLoader) LoadResource(res *Resource) error {
	base := filepath.Base(res.FullPath)
	res.IsIndex = base == "index.htm" || base == "_index.htm" || base == "index.html" || base == "_index.html"
	res.NeedsIndex = strings.HasSuffix(res.FullPath, ".htm") || strings.HasSuffix(res.FullPath, ".html")

	base = filepath.Base(res.WithoutExt(true))
	res.IsParametric = base[0] == '[' && base[len(base)-1] == ']'

	// if we are not parametric - then created the destination page
	if !res.IsParametric {
		res.DestPage = &Page{Res: res, Site: res.Site}
		res.DestPage.LoadFrom(res)
	} else {
		// what
	}
	return nil
}

func (m *HTMLResourceLoader) LoadParamValues(res *Resource) error {
	content, err := res.ReadAll()

	tmpl, err := res.Site.TextTemplateClone().Funcs(map[string]any{}).Parse(string(content))
	if err != nil {
		log.Println("Error parsing template: ", err, res.FullPath)
		return err
	}
	output := bytes.NewBufferString("")
	err = tmpl.Execute(output, &MDView{Res: res})
	if err != nil {
		log.Println("Error executing paramvals template: ", err, res.FullPath)
		return err
	} else {
		log.Println("Param Values After: ", res.ParamValues, output)
	}
	return err
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
	return SetNestedProp(page.RootView, view, location)
}

// A view that renders a Markdown
type HTMLView struct {
	BaseView[*Site]

	// Page we are rendering into
	Page *Page

	// Actual resource to render
	Res *Resource
}

func (v *HTMLView) RenderResponse(writer io.Writer) (err error) {
	res := v.Res
	mdfile, _ := res.Reader()
	mddata, _ := io.ReadAll(mdfile)
	defer mdfile.Close()

	mdTemplate, err := v.Context.HtmlTemplateClone().Parse(string(mddata))
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
