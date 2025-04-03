package s3gen

import (
	"bytes"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"

	gotl "github.com/panyam/templar"
)

type HTMLResourceLoader struct {
}

func NewHTMLResourceLoader() *HTMLResourceLoader {
	return &HTMLResourceLoader{}
}

func (m *HTMLResourceLoader) Load(r *Resource) error {
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

	finalhtml := bytes.NewBufferString("")
	err = r.Site.Templates.RenderHtmlTemplate(finalhtml, template, "", params, nil)
	if err != nil {
		log.Println("Error loading html template content: ", err, r.FullPath)
		return err
	}
	// and parse it markdown content

	r.Document.Loaded = true
	r.Document.LoadedAt = time.Now()
	r.Document.Root = finalhtml.String()

	// Other bookkeeping
	base := filepath.Base(r.FullPath)
	r.IsIndex = base == "index.htm" || base == "_index.htm" || base == "index.html" || base == "_index.html"
	r.NeedsIndex = strings.HasSuffix(r.FullPath, ".htm") || strings.HasSuffix(r.FullPath, ".html")

	base = filepath.Base(r.WithoutExt(true))
	r.IsParametric = base[0] == '[' && base[len(base)-1] == ']'

	// TODO - this needs to go - nothing magical about "Page"
	r.Site.CreateResourceBase(r)
	return nil
}

type HTMLResourceRenderer struct {
	BaseResourceRenderer
}

func NewHTMLResourceRenderer() (out *HTMLResourceRenderer) {
	out = &HTMLResourceRenderer{}
	return out
}

// A rule that converts <ContentRoot>/a/b/c.md -> <OutputDir>/a/b/c/index.html
// by Applying the root template defined in c.md as is
type HTMLToHtml struct {
	BaseToHtmlRule
	Loader              HTMLResourceLoader
	Renderer            HTMLResourceRenderer
	DefaultBaseTemplate string
}

func (m *HTMLToHtml) TargetsFor(s *Site, r *Resource) (siblings []*Resource, targets []*Resource) {
	m.Loader.Load(r)
	return m.BaseToHtmlRule.TargetsFor(s, r)
}

// Generate the output resource for a related set of "input" targets
func (m *HTMLToHtml) Run(site *Site, inputs []*Resource, targets []*Resource) error {
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
	}
	defer outfile.Close()

	tmpl, err := outres.Site.Templates.Loader.Load(template.Name, "")
	if err != nil {
		panic(err)
		// return err
	}

	params := map[any]any{
		"Site":        site,
		"Res":         inres,
		"FrontMatter": inres.FrontMatter().Data,
		"Content":     inres.Document.Root,
	}
	if template.Params != nil {
		maps.Copy(params, template.Params)
	}

	err = outres.Site.Templates.RenderHtmlTemplate(outfile, tmpl[0], template.Entry, params, nil)
	if err != nil {
		log.Println("Template: ", template)
		log.Println("Error rendering template: ", outres.FullPath, template, err)
		log.Println("Contents: ", string(tmpl[0].RawSource))
		_, err = outfile.Write(fmt.Appendf(nil, "Template error: %s", err.Error()))
	}
	return err
}
