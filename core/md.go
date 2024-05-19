package core

import (
	"bytes"
	"io"
	"log"
	"log/slog"
	"path/filepath"
	"strings"
	ttmpl "text/template"

	gut "github.com/panyam/goutils/utils"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

type MDContentProcessor struct {
	Template *ttmpl.Template
}

func NewMDContentProcessor(templatesDir string) *MDContentProcessor {
	h := &MDContentProcessor{}
	h.Template = ttmpl.New("hello")
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

func (m *MDContentProcessor) IsIndex(s *Site, res *Resource) bool {
	base := filepath.Base(res.FullPath)
	return base == "index.md" || base == "_index.md" || base == "index.mdx" || base == "_index.mdx"
}

// Returns a list of output resources that depend on this resource
func (m *MDContentProcessor) NeedsIndex(s *Site, res *Resource) bool {
	return strings.HasSuffix(res.FullPath, ".md") || strings.HasSuffix(res.FullPath, ".mdx")
}

func (m *MDContentProcessor) Process(s *Site, res *Resource, writer io.Writer) error {
	mdfile, _ := res.Reader()
	mddata, _ := io.ReadAll(mdfile)
	defer mdfile.Close()

	mdTemplate, err := m.Template.Parse(string(mddata))
	if err != nil {
		log.Println("Template Parse Error: ", err)
		return err
	}

	finalmd := bytes.NewBufferString("")
	err = mdTemplate.Execute(finalmd, gut.StringMap{
		"Site": s,
		"Page": res.FrontMatter,
	})
	if err != nil {
		log.Println("Error executing MD: ", err)
	}

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
		),
	)
	var buf bytes.Buffer
	if err := md.Convert(finalmd.Bytes(), &buf); err != nil {
		log.Println("error converting md: ", err)
		return err
	}

	// Now load the layout
	frontMatter := res.FrontMatter()
	layoutName := ""
	if frontMatter != nil && frontMatter.Data != nil {
		if frontMatter.Data["layout"] != "" {
			if layout, ok := frontMatter.Data["layout"].(string); ok {
				layoutName = layout
			}
		}
	}
	if layoutName == "" { // If we dont have a layout name then render as is
		writer.Write(buf.Bytes())
	} else {
		// out2 := bytes.NewBufferString("")
		err = s.HtmlTemplate.ExecuteTemplate(writer, layoutName, map[string]any{
			"Content":      string(buf.Bytes()),
			"SiteMetadata": s.SiteMetadata,
			"PrevPost":     nil,
			"NextPost":     nil,
			"Post": map[string]any{
				"Slug": "testslug",
			},
		})
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
func (m *MDContentProcessor) PopulatePage(res *Resource, page *Page) error {
	site := page.Site
	frontMatter := res.FrontMatter().Data
	pageName := "BasePage"
	if frontMatter["page"] != nil && frontMatter["page"] != "" {
		pageName = frontMatter["page"].(string)
	}
	page.RootView = site.NewPageView(pageName)
	page.RootView.SetPage(page)

	location := "BodyView"
	if frontMatter["location"] != nil {
		location = frontMatter["location"].(string)
	}

	mdview := &MDView{Res: res}
	// log.Println("Before pageName, location: ", pageName, location, page.RootView, mdview)
	// defer log.Println("After pageName, location: ", pageName, location, page.RootView)
	return SetViewProp(page.RootView, mdview, location)
}

// A view that renders a Markdown
type MDView struct {
	BaseView

	// Actual resource to render
	Res *Resource
}

func (v *MDView) InitContext(site *Site, parentView View) {
	if v.Self == nil {
		v.Self = v
	}
	v.BaseView.InitContext(site, parentView)
}

func (v *MDView) RenderResponse(writer io.Writer) (err error) {
	res := v.Res
	mdfile, _ := res.Reader()
	mddata, _ := io.ReadAll(mdfile)
	defer mdfile.Close()

	mdTemplate, err := v.Site.TextTemplate.Parse(string(mddata))
	if err != nil {
		slog.Error("Template Parse Error: ", "error", err)
		return err
	}

	finalmd := bytes.NewBufferString("")
	err = mdTemplate.Execute(finalmd, gut.StringMap{
		"Site": v.Site,
		"Page": res.FrontMatter,
	})
	if err != nil {
		slog.Error("Error executing MD: ", "error", err)
	}

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
		),
	)
	var buf bytes.Buffer
	if err := md.Convert(finalmd.Bytes(), &buf); err != nil {
		slog.Error("error converting md: ", "error", err)
		return err
	}

	// Now load the layout
	frontMatter := res.FrontMatter()
	layoutName := ""
	if frontMatter != nil && frontMatter.Data != nil {
		if frontMatter.Data["layout"] != "" {
			if layout, ok := frontMatter.Data["layout"].(string); ok {
				layoutName = layout
			}
		}
	}
	if true || layoutName == "" { // If we dont have a layout name then render as is
		writer.Write(buf.Bytes())
	} else {
		// out2 := bytes.NewBufferString("")
		err = v.Site.HtmlTemplate.ExecuteTemplate(writer, layoutName, map[string]any{
			"Content":  string(buf.Bytes()),
			"PrevPost": nil,
			"NextPost": nil,
			"Post": map[string]any{
				"Slug": "testslug",
			},
		})
	}
	return err
}
