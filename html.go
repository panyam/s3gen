package s3gen

import (
	"bytes"
	"fmt"
	"log"
	"log/slog"
	"maps"
	"os"

	gotl "github.com/panyam/templar"
)

// A rule that converts <ContentRoot>/a/b/c.html -> <OutputDir>/a/b/c/index.html
// by applying the root template defined in c.html as is
type HTMLToHtml struct {
	BaseToHtmlRule
}

// Phase returns PhaseGenerate - HTML template conversion happens in the generate phase.
func (h *HTMLToHtml) Phase() BuildPhase {
	return PhaseGenerate
}

// DependsOn returns nil - HTMLToHtml doesn't depend on other rules.
func (h *HTMLToHtml) DependsOn() []string {
	return nil
}

// Produces returns the patterns of files this rule generates.
func (h *HTMLToHtml) Produces() []string {
	return []string{"**/*.html"}
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
	if funcs == nil {
		funcs = map[string]any{}
	}
	maps.Copy(funcs, map[string]any{
		"OurContent": func() string {
			finalmd := tmpl[0].RawSource
			log.Println("Calling OurContent: ", len(finalmd), inres.FullPath)
			return string(finalmd)
		},
		"MDToHtml": func(input any) string {
			return "TBD"
		},
	})

	// log.Println("1111 ---- Rendering HTML with Template", "outres", outres.FullPath, "template", template.Name, "entry", template.Entry)
	slog.Debug("Rendering with Template", "inres", inres.FullPath, "template", template.Name, "entry", template.Entry)
	err = outres.Site.Templates.RenderHtmlTemplate(outfile, tmpl[0], template.Entry, params, funcs)
	if err != nil {
		log.Println("Error rendering template: ", outres.FullPath, template, err)
		log.Println("Contents: ", string(tmpl[0].RawSource))
		_, err = outfile.Write(fmt.Appendf(nil, "HTML Template error: %s", err.Error()))
	}
	return panicOrError(err)
}

func (h *HTMLToHtml) LoadResourceTemplate(site *Site, r *Resource) ([]byte, error) {
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

	// Include AssetURL function for co-located asset references
	funcs := map[string]any{
		"AssetURL": func(filename string) string {
			return GetAssetURL(r.Site, r, filename)
		},
	}

	finalmd := bytes.NewBufferString("")
	err = r.Site.Templates.RenderHtmlTemplate(finalmd, template, "", params, funcs)
	if err != nil {
		log.Println("Error loading template content: ", err, r.FullPath)
		return nil, err
	}

	return finalmd.Bytes(), nil
}
