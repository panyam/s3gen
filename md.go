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

type MDResourceHandler struct {
	defaultResourceHandler
	Template *ttmpl.Template
}

func NewMDResourceHandler(templatesDir string) *MDResourceHandler {
	h := &MDResourceHandler{}
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

func (m *MDResourceHandler) LoadResource(res *Resource) error {
	// Make sure resource's front matter is loaded if any
	res.FrontMatter()

	base := filepath.Base(res.FullPath)
	res.IsIndex = base == "index.md" || base == "_index.md" || base == "index.mdx" || base == "_index.mdx"
	res.NeedsIndex = strings.HasSuffix(res.FullPath, ".md") || strings.HasSuffix(res.FullPath, ".mdx")

	base = filepath.Base(res.WithoutExt(true))
	res.IsParametric = base[0] == '[' && base[len(base)-1] == ']'

	// if we are not parametric - then created the destination page
	res.Site.CreatePage(res)
	// res.Page.LoadFrom(res)
	return nil
}

func (m *MDResourceHandler) LoadParamValues(res *Resource) error {
	output := bytes.NewBufferString("")
	err := m.RenderContent(res, output)
	if err != nil {
		log.Println("Error executing paramvals template: ", err, res.FullPath)
	} else {
		log.Println("Param Values After: ", res.ParamValues, output)
	}
	return err
}

// Renders just the content section within the resource
func (m *MDResourceHandler) RenderContent(res *Resource, w io.Writer) error {
	mddata, _ := res.ReadAll()

	site := res.Site
	mdTemplate, err := site.TextTemplate().Parse(string(mddata))
	if err != nil {
		slog.Error("Template Parse Error: ", "error", err)
		return err
	}

	finalmd := bytes.NewBufferString("")
	err = mdTemplate.Execute(finalmd, map[string]any{
		"Site": site,
		"Res":  res,
	})
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

	_, err = w.Write(buf.Bytes())
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
