package s3gen

import (
	"bytes"
	"fmt"
	htmpl "html/template"
	"log"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"

	gotl "github.com/panyam/templar"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/anchor"
)

type MDResourceLoader struct {
}

func NewMDResourceLoader() *MDResourceLoader {
	return &MDResourceLoader{}
}

func (m *MDResourceLoader) Load(r *Resource) error {
	r.FrontMatter()
	if r.Error != nil {
		return r.Error
	}

	// Load the rest of the content so we can parse it
	source, err := r.ReadAll()
	if err != nil {
		return err
	}

	template := &gotl.Template{
		RawSource: source,
		Path:      r.FullPath,
		AsHtml:    true,
	}

	params := map[any]any{
		"Res":  r,
		"Site": r.Site,
		// "FrontMatter": res.FrontMatter().Data,
	}

	finalmd := bytes.NewBufferString("")
	err = r.Site.Templates.RenderTextTemplate(finalmd, template, "", params, nil)
	if err != nil {
		log.Println("Error loading template content: ", err, r.FullPath)
		return err
	}

	// and parse it markdown content
	tocTransformer := NewTOCTransformer()
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Strikethrough,
			extension.Typographer,
			highlighting.NewHighlighting(
				highlighting.WithStyle("monokai"),
				highlighting.WithFormatOptions(
				// chromahtml.WithLineNumbers(true),
				),
			),
			&anchor.Extender{},
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(
				// util.Prioritized(&preCodeWrapper{}, 100),
				util.PrioritizedValue{
					Value:    tocTransformer,
					Priority: 100,
				},
			),
		),
	)

	r.Document.Loaded = true
	r.Document.LoadedAt = time.Now()
	r.Document.SetMetadata("TOC", tocTransformer.TOC)
	doc := md.Parser().Parse(text.NewReader(source))
	log.Println("Did we parse this: ", r.FullPath, doc)
	r.Document.Root = doc

	// Other basic book keeping
	base := filepath.Base(r.FullPath)
	r.IsIndex = base == "index.md" || base == "_index.md" || base == "index.mdx" || base == "_index.mdx"
	r.NeedsIndex = strings.HasSuffix(r.FullPath, ".md") || strings.HasSuffix(r.FullPath, ".mdx")

	base = filepath.Base(r.WithoutExt(true))
	r.IsParametric = base[0] == '[' && base[len(base)-1] == ']'

	// TODO - this needs to go - nothing magical about "Page"
	r.Site.CreateResourceBase(r)

	return nil
}

// A rule that converts <ContentRoot>/a/b/c.md -> <OutputDir>/a/b/c/index.html
// by Applying the root template defined in c.md as is
type MDToHtml struct {
	BaseToHtmlRule
	Loader              MDResourceLoader
	DefaultBaseTemplate string
}

func (m *MDToHtml) MD() (md goldmark.Markdown, tocTransformer *TOCTransformer) {
	tocTransformer = NewTOCTransformer()
	md = goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Strikethrough,
			extension.Typographer,
			highlighting.NewHighlighting(
				highlighting.WithStyle("monokai"),
				highlighting.WithFormatOptions(
				// chromahtml.WithLineNumbers(true),
				),
			),
			&anchor.Extender{},
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(
				// util.Prioritized(&preCodeWrapper{}, 100),
				util.PrioritizedValue{
					Value:    tocTransformer,
					Priority: 100,
				},
			),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			html.WithUnsafe(),
		),
	)
	return
}

func (m *MDToHtml) TargetsFor(s *Site, r *Resource) (siblings []*Resource, targets []*Resource) {
	m.Loader.Load(r)
	return m.BaseToHtmlRule.TargetsFor(s, r)
}

func (m *MDToHtml) loadContent(md goldmark.Markdown, r *Resource) ([]byte, error) {
	r.FrontMatter()
	if r.Error != nil {
		return nil, r.Error
	}

	// Load the rest of the content so we can parse it
	source, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	template := &gotl.Template{
		RawSource: source,
		Path:      r.FullPath,
		AsHtml:    true,
	}

	params := map[any]any{
		"Res":  r,
		"Site": r.Site,
		// "FrontMatter": res.FrontMatter().Data,
	}

	finalmd := bytes.NewBufferString("")
	err = r.Site.Templates.RenderTextTemplate(finalmd, template, "", params, nil)
	if err != nil {
		log.Println("Error loading template content: ", err, r.FullPath)
		return nil, err
	}
	return finalmd.Bytes(), err
}

// Generate the output resource for a related set of "input" targets
func (m *MDToHtml) Run(site *Site, inputs []*Resource, targets []*Resource) error {
	if len(inputs) != 1 || len(targets) != 1 {
		// This rule can only match 1 input to one output
		panic(fmt.Errorf("Exactly 1 input and output needed, found %d, %d", len(inputs), len(targets)))
	}

	inres := inputs[0]
	template, err := m.getResourceTemplate(inres)
	if err != nil {
		return err
	}

	outres := targets[0]
	outres.EnsureDir()
	outfile, err := os.Create(outres.FullPath)
	if err != nil {
		log.Println("Error writing to: ", outres.FullPath, err)
		return err
	}
	defer outfile.Close()

	tmpl, err := outres.Site.Templates.Loader.Load(template.Name, "")
	if err != nil {
		panic(err)
		// return err
	}

	md, _ := m.MD()

	params := map[any]any{
		"Site":        site,
		"Res":         inres,
		"FrontMatter": inres.FrontMatter().Data,
		"Content":     inres.Document.Root,
	}
	if template.Params != nil {
		maps.Copy(params, template.Params)
	}

	log.Println("Entry Template: ", template.Entry, params)
	err = outres.Site.Templates.RenderHtmlTemplate(outfile, tmpl[0], template.Entry, params, map[string]any{
		"ParseMD": func(content []byte) (*struct {
			Doc *ast.Document
			TOC []*TOCNode
		}, error) {
			return nil, nil
		},
		"MDToHtml": func(doc *ast.Document) (htmpl.HTML, error) {
			var b bytes.Buffer
			src, err := inres.ReadAll()
			if err != nil {
				return htmpl.HTML(err.Error()), err
			}
			err = md.Renderer().Render(&b, src, doc)
			log.Println("Called md2html: ", doc)
			log.Println("Error: ", err)
			return htmpl.HTML(b.String()), err
		},
	})
	if err != nil {
		log.Println("Template: ", template)
		log.Println("Error rendering template: ", outres.FullPath, template, err)
		log.Println("Contents: ", string(tmpl[0].RawSource))
		_, err = outfile.Write(fmt.Appendf(nil, "Template error: %s", err.Error()))
	}
	return err
}
