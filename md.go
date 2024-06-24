package s3gen

import (
	"bytes"
	"io"
	"log"
	"log/slog"
	"path/filepath"
	"strings"
	ttmpl "text/template"

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

type MDResourceLoader struct {
	DefaultResourceLoader
	Template *ttmpl.Template
}

func NewMDResourceLoader(templatesDir string) *MDResourceLoader {
	h := &MDResourceLoader{}
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

func (m *MDResourceLoader) LoadResource(res *Resource) error {
	base := filepath.Base(res.FullPath)
	res.IsIndex = base == "index.md" || base == "_index.md" || base == "index.mdx" || base == "_index.mdx"
	res.NeedsIndex = strings.HasSuffix(res.FullPath, ".md") || strings.HasSuffix(res.FullPath, ".mdx")

	base = filepath.Base(res.WithoutExt(true))
	res.IsParametric = base[0] == '[' && base[len(base)-1] == ']'

	// if we are not parametric - then created the destination page
	if true || !res.IsParametric {
		res.DestPage = &Page{Res: res, Site: res.Site}
		res.DestPage.LoadFrom(res)
	}
	return nil
}

func (m *MDResourceLoader) LoadParamValues(res *Resource) error {
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
		log.Println("Param Values After: ", res.ParamValues)
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
func (m *MDResourceLoader) SetupPageView(res *Resource, page *Page) (err error) {
	// log.Println("RelPath, Link: ", relpath, page.Link)
	frontMatter := res.FrontMatter().Data
	location := "BodyView"
	if frontMatter["location"] != nil {
		location = frontMatter["location"].(string)
	}

	mdview := &MDView{Res: res, Page: page}
	// log.Println("Before pageName, location: ", pageName, location, page.RootView, mdview)
	// defer log.Println("After pageName, location: ", pageName, location, page.RootView)
	return SetNestedProp(page.RootView, mdview, location)
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

	mdTemplate, err := v.Site.TextTemplate().Parse(string(mddata))
	if err != nil {
		slog.Error("Template Parse Error: ", "error", err)
		return err
	}

	finalmd := bytes.NewBufferString("")
	err = mdTemplate.Execute(finalmd, v)
	if err != nil {
		log.Println("Error executing MD: ", res.FullPath, err)
	}

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Typographer,
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
				util.Prioritized(&preCodeWrapper{}, 100),
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

	writer.Write(buf.Bytes())
	return err
}

// A goldmark AST transformer that wraps the <pre> block inside a div that allows copy-pasting
// of underlying code
type preCodeWrapper struct {
}

func (t *preCodeWrapper) Transform(doc *ast.Document, reader text.Reader, ctx parser.Context) {
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
