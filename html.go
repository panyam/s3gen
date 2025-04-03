package s3gen

import (
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"strings"
)

// A rule that converts <ContentRoot>/a/b/c.md -> <OutputDir>/a/b/c/index.html
// by Applying the root template defined in c.md as is
type HTMLToHtml struct {
	BaseToHtmlRule
	DefaultBaseTemplate string
}

func (h *HTMLToHtml) TargetsFor(s *Site, r *Resource) (siblings []*Resource, targets []*Resource) {
	h.LoadResource(s, r)
	return h.BaseToHtmlRule.TargetsFor(s, r)
}

// Generate the output resource for a related set of "input" targets
func (m *HTMLToHtml) Run(site *Site, inputs []*Resource, targets []*Resource, funcs map[string]any) error {
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
		log.Println("Error rendering template: ", outres.FullPath, template, err)
		log.Println("Contents: ", string(tmpl[0].RawSource))
		_, err = outfile.Write(fmt.Appendf(nil, "HTML Template error: %s", err.Error()))
	}
	return panicOrError(err)
}

func (h *HTMLToHtml) LoadResource(site *Site, r *Resource) error {
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
