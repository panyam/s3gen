package core

import (
	"bytes"
	"io"
	"log"
	"log/slog"
	"path/filepath"
	"strings"
	ttmpl "text/template"
	"time"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/anchor"
)

type DefaultContentProcessor struct {
}

func (d *DefaultContentProcessor) LoadPageFromMatter(page *Page, frontMatter map[string]any) {
	pageName := "BasePage"
	if frontMatter["page"] != nil && frontMatter["page"] != "" {
		pageName = frontMatter["page"].(string)
	}
	site := page.Site
	page.RootView = site.NewView(pageName)
	page.RootView.SetPage(page)

	// For now we are going through "known" fields
	// TODO - just do this by dynamically going through all fields in FM
	// and calling SetViewProps and fail if this field doesnt exist - or using struct tags
	var err error
	if val, ok := frontMatter["title"]; val != nil && ok {
		page.Title = val.(string)
	}
	if val, ok := frontMatter["summary"]; val != nil && ok {
		page.Summary = val.(string)
	}
	if val, ok := frontMatter["date"]; val != nil && ok {
		// create at
		if val.(string) != "" {
			if page.CreatedAt, err = time.Parse("2006-1-2T03:04:05PM", val.(string)); err != nil {
				log.Println("error parsing created time: ", err)
			}
		}
	}

	if val, ok := frontMatter["lastmod"]; val != nil && ok {
		// update at
		if val.(string) != "" {
			if page.UpdatedAt, err = time.Parse("2006-1-2", val.(string)); err != nil {
				log.Println("error parsing last mod time: ", err)
			}
		}
	}

	if val, ok := frontMatter["draft"]; val != nil && ok {
		// update at
		page.IsDraft = val.(bool)
	}
}

type MDContentProcessor struct {
	DefaultContentProcessor
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
func (m *MDContentProcessor) LoadPage(res *Resource, page *Page) error {
	frontMatter := res.FrontMatter().Data
	m.LoadPageFromMatter(page, frontMatter)

	location := "BodyView"
	if frontMatter["location"] != nil {
		location = frontMatter["location"].(string)
	}

	mdview := &MDView{Res: res, Page: page}
	// log.Println("Before pageName, location: ", pageName, location, page.RootView, mdview)
	// defer log.Println("After pageName, location: ", pageName, location, page.RootView)
	return SetViewProp(page.RootView, mdview, location)
}

// A view that renders a Markdown
type MDView struct {
	BaseView

	// Page we are rendering into
	Page *Page

	// Actual resource to render
	Res *Resource
}

func (v *MDView) InitView(site *Site, parentView View) {
	if v.Self == nil {
		v.Self = v
	}
	v.BaseView.InitView(site, parentView)
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
	err = mdTemplate.Execute(finalmd, v)
	if err != nil {
		slog.Error("Error executing MD: ", "error", err)
	}

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("monokai"),
				highlighting.WithFormatOptions(
					chromahtml.WithLineNumbers(true),
				),
			),
			&anchor.Extender{},
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(
				util.Prioritized(&PreCodeWrapper{}, 100),
			),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			html.WithUnsafe(),
		),
	)

	// TODO - Any tree processing/transforms etc here
	// node := md.Parser().Parse(text.NewReader(finalmd.Bytes()))
	// log.Println("Parsed Node: ", node, node.Kind())

	var buf bytes.Buffer
	if err := md.Convert(finalmd.Bytes(), &buf); err != nil {
		slog.Error("error converting md: ", "error", err)
		return err
	}

	if true {
		writer.Write(buf.Bytes())
	} else {
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
			err = v.Site.HtmlTemplate.ExecuteTemplate(writer, layoutName, map[string]any{
				"Content":  string(buf.Bytes()),
				"PrevPost": nil,
				"NextPost": nil,
				"Post": map[string]any{
					"Slug": "testslug",
				},
			})
		}
	}
	return err
}

// A goldmark AST transformer that wraps the <pre> block inside a div that allows copy-pasting
// of underlying code
type PreCodeWrapper struct {
}

func (t *PreCodeWrapper) Transform(doc *ast.Document, reader text.Reader, ctx parser.Context) {
	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		// log.Println("Entering: ", n)
		return 0, nil
	})

	if err != nil {
		log.Println("Walk Error: ", err)
	}
}
