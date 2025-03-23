package s3gen

import (
	"bytes"
	"log"
	"path/filepath"
	"strings"
	"time"

	gotl "github.com/panyam/templar"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
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
	doc := md.Parser().Parse(text.NewReader(source))
	r.Document.Root = doc

	// Other basic book keeping
	base := filepath.Base(r.FullPath)
	r.IsIndex = base == "index.md" || base == "_index.md" || base == "index.mdx" || base == "_index.mdx"
	r.NeedsIndex = strings.HasSuffix(r.FullPath, ".md") || strings.HasSuffix(r.FullPath, ".mdx")

	base = filepath.Base(r.WithoutExt(true))
	r.IsParametric = base[0] == '[' && base[len(base)-1] == ']'

	// TODO - this needs to go - nothing magical about "Page"
	r.Site.CreatePage(r)

	return nil
}

type MDResourceRenderer struct {
	BaseResourceRenderer
	md goldmark.Markdown
}

func NewMDResourceRenderer() (out *MDResourceRenderer) {
	out = &MDResourceRenderer{}
	out.md = goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Typographer,
			highlighting.NewHighlighting(
				highlighting.WithStyle("monokai"),
				highlighting.WithFormatOptions(
				// chromahtml.WithLineNumbers(true),
				),
			),
			&anchor.Extender{},
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			html.WithUnsafe(),
		),
	)
	return out
}
