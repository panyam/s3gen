package s3gen

import (
	"bytes"
	"fmt"
	htmpl "html/template"
	"log"
	"log/slog"
	"maps"
	"os"

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

// A rule that converts <ContentRoot>/a/b/c.md -> <OutputDir>/a/b/c/index.html
// by Applying the root template defined in c.md as is
type MDToHtml struct {
	BaseToHtmlRule
}

// Phase returns PhaseGenerate - markdown conversion happens in the generate phase.
func (m *MDToHtml) Phase() BuildPhase {
	return PhaseGenerate
}

// DependsOn returns nil - MDToHtml doesn't depend on other rules.
func (m *MDToHtml) DependsOn() []string {
	return nil
}

// Produces returns the patterns of files this rule generates.
func (m *MDToHtml) Produces() []string {
	return []string{"**/*.html"}
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
				highlighting.WithWrapperRenderer(func(w util.BufWriter, ctx highlighting.CodeBlockContext, entering bool) {
					lang, ok := ctx.Language()
					if ok && string(lang) == "mermaid" {
						if entering {
							w.WriteString(`<pre class="mermaid">`)
						} else {
							w.WriteString(`</pre>`)
						}
						return
					}
					// Default behavior for other languages
					if entering {
						w.WriteString(`<pre><code>`)
					} else {
						w.WriteString(`</code></pre>`)
					}
				}),
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
	m.LoadResource(s, r)
	return m.BaseToHtmlRule.TargetsFor(s, r)
}

// Generate the output resource for a related set of "input" targets
func (m *MDToHtml) Run(site *Site, inputs []*Resource, targets []*Resource, funcs map[string]any) error {
	if len(inputs) != 1 || len(targets) != 1 {
		// This rule can only match 1 input to one output
		return panicOrError(fmt.Errorf("Exactly 1 input and output needed, found %d, %d", len(inputs), len(targets)))
	}

	inres := inputs[0]
	template, err := m.getResourceTemplate(inres)
	if err != nil {
		return panicOrError(err)
	}

	tmpl, err := site.Templates.Loader.Load(template.Name, "")
	if err != nil {
		return panicOrError(err)
	}

	outres := targets[0]
	outres.EnsureDir()
	outfile, err := os.Create(outres.FullPath)
	if err != nil {
		log.Println("Error writing to: ", outres.FullPath, err)
		return panicOrError(err)
	}
	defer outfile.Close()

	finalmd, err := m.LoadResourceTemplate(site, inres)

	params := map[any]any{
		"Site":        site,
		"Res":         inres,
		"FrontMatter": inres.FrontMatter().Data,
		"Content":     finalmd,
	}
	if template.Params != nil {
		maps.Copy(params, template.Params)
	}

	md, tocTransformer := m.MD()

	if funcs == nil {
		funcs = map[string]any{}
	}
	maps.Copy(funcs, map[string]any{
		"OurContent": func() string {
			log.Println("Calling OurContent: ", len(finalmd), inres.FullPath)
			return string(finalmd)
		},
		"ParseMD": func(content []byte) (*struct {
			Doc *ast.Document
			TOC []TOCNode
		}, error) {
			// log.Println("Parsing Content: ", inres.FullPath, "ContentLen: ", len(content), "MDLen: ", len(finalmd))
			// log.Println(string(content))
			if len(content) != len(finalmd) {
				panic("Content and MD len do not match")
			}
			doc := md.Parser().Parse(text.NewReader(content))
			return &(struct {
				Doc *ast.Document
				TOC []TOCNode
			}{
				Doc: doc.(*ast.Document),
				TOC: tocTransformer.TOC,
			}), nil
		},
		"MDToHtml": func(doc *ast.Document) (htmpl.HTML, error) {
			var b bytes.Buffer
			// log.Println("2222.... Did we come here???", len(finalmd))
			// log.Println("Rendering with Template", "inres", inres.FullPath, "outres", "template", template.Name, "entry", template.Entry)
			// log.Println("Rendering Content: ", len(finalmd))
			err := md.Renderer().Render(&b, finalmd, doc)
			if err != nil {
				panic(err)
			}
			return htmpl.HTML(b.String()), err
		},
	})

	// log.Println("1111 ---- Rendering with MD Template", "outres", outres.FullPath, "template", template.Name, "entry", template.Entry)
	// log.Println("3333 ---- Rendering with MD Template", "outres", outres.FullPath, "template", template.Name, "entry", template.Entry)
	slog.Debug("Rendering with Template", "inres", inres.FullPath, "template", template.Name, "entry", template.Entry)
	err = outres.Site.Templates.RenderHtmlTemplate(outfile, tmpl[0], template.Entry, params, funcs)
	// log.Println("Finished Rendering, err: ", err)
	if err != nil {
		log.Println("Error rendering template: ", outres.FullPath, template, err)
		log.Println("Contents: ", string(tmpl[0].RawSource))
		_, err = outfile.Write(fmt.Appendf(nil, "MD Template error: %s", err.Error()))
	}
	return panicOrError(err)
}

func (m *MDToHtml) LoadResourceTemplate(site *Site, r *Resource) ([]byte, error) {
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
		"Res":         r,
		"Site":        r.Site,
		"FrontMatter": r.FrontMatter().Data,
	}

	finalmd := bytes.NewBufferString("")
	err = r.Site.Templates.RenderTextTemplate(finalmd, template, "", params, nil)
	if err != nil {
		log.Println("Error loading template content: ", err, r.FullPath)
		return nil, err
	}

	return finalmd.Bytes(), nil
}
