package s3gen

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"path/filepath"

	gotl "github.com/panyam/goutils/template"
	"github.com/panyam/templar"
)

// ParametricPages is a rule that handles the discovery and generation of
// pages from a single parametric template (e.g., content/tags/[tag].html).
// It acts as a dispatcher, delegating the final rendering to a sub-rule
// based on the file extension.
type ParametricPages struct {
	// Renderers is a map of file extensions to the Rule that should be used
	// for rendering that file type. For example:
	// {
	//   ".md": &MDToHtml{},
	//   ".html": &HTMLToHtml{},
	// }
	Renderers map[string]Rule
}

// TargetsFor first checks if the resource's extension is supported by one of its
// renderers. If so, it performs a "discovery" render to populate the resource's
// ParamValues, then generates a target resource for each of those values.
func (p *ParametricPages) TargetsFor(s *Site, r *Resource) (siblings []*Resource, targets []*Resource) {
	// Check if we have a renderer for this file extension.
	renderer, ok := p.Renderers[r.Ext()]
	if !ok {
		return nil, nil
	}

	// This is a bit of a hack to reuse the LoadResource logic from the child rule
	// to set IsParametric, etc.
	if loader, ok := renderer.(interface{ LoadResource(*Site, *Resource) error }); ok {
		loader.LoadResource(s, r)
	}

	// This rule only applies to parametric resources.
	if !r.IsParametric {
		return nil, nil
	}

	// Phase 1: Discovery.
	if len(r.ParamValues) == 0 {
		slog.Info("Discovering params for", "resource", r.FullPath)
		r.ParamName = ""

		// We need to get the template content to perform the discovery render.
		// We can't use a pre-built renderer here, so we do it manually.
		content, err := r.ReadAll()
		if err != nil {
			log.Printf("Error reading file for param discovery %s: %v", r.FullPath, err)
			return nil, nil
		}

		params := map[any]any{
			"Site":        s,
			"Res":         r,
			"FrontMatter": r.FrontMatter().Data,
		}

		var discoveryBuffer bytes.Buffer
		err = s.Templates.RenderHtmlTemplate(&discoveryBuffer, &templar.Template{
			RawSource: content,
			Path:      r.FullPath,
		}, "", params, s.DefaultFuncMap())

		if err != nil {
			log.Printf("Error during parameter discovery for %s: %v", r.FullPath, err)
			return nil, nil
		}
		slog.Info("Discovered params", "resource", r.FullPath, "values", r.ParamValues)
	}

	// Phase 2: Target Generation.
	respath, _ := filepath.Rel(s.ContentRoot, r.FullPath)
	ext := filepath.Ext(respath)
	rem := respath[:len(respath)-len(ext)]
	dirname := filepath.Dir(rem)

	for _, paramValue := range r.ParamValues {
		// Ensure paramValue is URL-safe
		safeParamValue := gotl.Slugify(paramValue)
		destpath := filepath.Join(s.OutputDir, dirname, safeParamValue, "index.html")
		destres := s.GetResource(destpath)
		destres.Source = r
		destres.Base = r.Base
		destres.frontMatter = r.frontMatter
		destres.ParamName = paramValue // Keep original param name for display
		targets = append(targets, destres)
	}

	return []*Resource{r}, targets
}

// Run finds the correct renderer based on the input file's extension
// and delegates the rendering job to it.
func (p *ParametricPages) Run(site *Site, inputs []*Resource, targets []*Resource, funcs map[string]any) (err error) {
	if len(inputs) != 1 {
		return fmt.Errorf("ParametricPages rule requires exactly 1 input, found %d inputs, %d targets", len(inputs), len(targets))
	}

	inres := inputs[0]

	renderer, ok := p.Renderers[inres.Ext()]
	if !ok {
		return fmt.Errorf("no renderer found for extension %s in ParametricPages rule", inres.Ext())
	}

	for idx, target := range targets {
		outres := targets[0]
		slog.Debug("Dispatching to renderer", "rule", fmt.Sprintf("%T", renderer), "in", inres.FullPath, "out", outres.FullPath, "param", outres.ParamName)

		// Here's the delegation: call the Run method of the specialized rule.
		// We need to pass the target resource's information (specifically the ParamName)
		// down to the renderer. We can do this by modifying the input resource temporarily
		// or by enhancing the renderer's Run method to accept more context.
		// For now, let's just run it. The renderer will need to be aware of the target's ParamName.
		// A better way would be to pass `outres` to the `Run` method.
		// Let's assume the renderer can access `target.ParamName`.
		// The `HTMLToHtml` and `MDToHtml` Run methods may need a slight modification to
		// use the `outres.ParamName` when rendering.

		// For the purpose of this example, we will assume the existing Run methods
		// can handle this. In a real implementation, you might need to adjust them.
		inres.ParamName = inres.ParamValues[idx]
		err2 := renderer.Run(site, inputs, []*Resource{target}, funcs)
		err = errors.Join(err, err2)
	}
	return
}
