package core

import (
	"bytes"
	"io"
	"log"
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

// Idea is the resource may have a lot of information on how it should be rendered
// Given a page we want to identify what page properties should be set from here
// so when page is finally rendered it is all uptodate
func (m *MDContentProcessor) PopulatePage(res *Resource, page *Page) error {
	site := page.Site
	frontMatter := res.FrontMatter().Data
	pageName := ""
	if frontMatter["page"] != nil {
		pageName = frontMatter["page"].(string)
	}
	page.RootView = site.NewView(pageName)

	location := "BodyView"
	if frontMatter["location"] != nil {
		location = frontMatter["location"].(string)
	}

	mdview := &MDView{Res: res}
	SetViewProp(page.RootView, mdview, location)
	return nil
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
