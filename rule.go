package s3gen

import (
	"log"
	"path/filepath"
	"slices"
	"strings"
)

// BaseTemplate defines the data needed to render a page, including the
// template name and any parameters it requires.
type BaseTemplate struct {
	// Name is the name of the template file to use as the root.
	Name string

	// Entry is the name of the template to use as the entry point within the
	// template file. This is useful when a single file contains multiple
	// template definitions.
	Entry string

	// Params is a map of parameters to pass to the template.
	Params map[any]any
}

// Rule is an interface that defines how to process a resource. s3gen's build
// system is based on a set of rules that are applied to resources in a specific
// order. This allows for a flexible and extensible build process.
type Rule interface {
	// TargetsFor determines if the rule can be applied to a given resource and,
	// if so, what the output file (or "target") should be. It can also identify
	// any "sibling" resources that are needed to process the input resource.
	TargetsFor(site *Site, res *Resource) (siblings []*Resource, targets []*Resource)

	// Run contains the logic for processing the resource. It takes the input
	// resources (the original resource and any siblings) and the target
	// resources and generates the output.
	Run(site *Site, inputs []*Resource, targets []*Resource, funcs map[string]any) error
}

// BaseToHtmlRule is a base rule that can be embedded in other rules that
// convert a resource to HTML.
type BaseToHtmlRule struct {
	// Extensions is a list of file extensions that this rule can handle.
	Extensions []string
}

// getResourceTemplate returns the template to use for a given resource.
func (m *BaseToHtmlRule) getResourceTemplate(res *Resource) (template BaseTemplate, err error) {
	frontMatter := res.FrontMatter().Data

	// Start with the default
	template = res.Site.DefaultBaseTemplate

	// which page template to use
	if res.Site.GetTemplate != nil {
		res.Site.GetTemplate(res, &template)
	}

	// now see if we can override them what is on the page
	if frontMatter["template"] != nil && frontMatter["template"] != "" {
		templateAndEntry := strings.Split(frontMatter["template"].(string), "/")
		template.Name = templateAndEntry[0]
		if len(templateAndEntry) > 1 {
			template.Entry = templateAndEntry[1]
		}
	}
	if frontMatter["templateParams"] != nil {
		template.Params = frontMatter["templateParams"].(map[any]any)
	}
	return
}

// TargetsFor determines the output target for a resource that is being
// converted to HTML.
func (m *BaseToHtmlRule) TargetsFor(s *Site, r *Resource) (siblings []*Resource, targets []*Resource) {
	respath, found := strings.CutPrefix(r.FullPath, s.ContentRoot)
	if !found {
		log.Println("Respath not found: ", r.FullPath, s.ContentRoot)
		return nil, nil
	}

	if !slices.Contains(m.Extensions, r.Ext()) {
		return
	}

	// log.Println("isValid, Res, Extensions, isParametric, isIndex, needsIndex: ", isValidExt, r.FullPath, m.Extensions, r.IsParametric, r.IsIndex, r.NeedsIndex)
	if r.IsParametric {
		ext := filepath.Ext(respath)

		rem := respath[:len(respath)-len(ext)]
		dirname := filepath.Dir(rem)

		// TODO - also see if there is a .<lang> prefix on rem after
		// ext has been removed can use that for language sites
		for _, paramName := range r.ParamValues {
			destpath := filepath.Join(s.OutputDir, dirname, paramName, "index.html")
			destres := s.GetResource(destpath)
			destres.Source = r
			destres.Base = r.Base
			destres.frontMatter = r.frontMatter
			destres.ParamName = paramName

			targets = append(targets, destres)
		}
	} else {
		// we have a basic resource so generate it
		destpath := ""
		if r.Info().IsDir() {
			// Then this will be served with dest/index.html
			destpath = filepath.Join(s.OutputDir, respath)
		} else if r.IsIndex {
			destpath = filepath.Join(s.OutputDir, filepath.Dir(respath), "index.html")
		} else if r.NeedsIndex {
			// res is not a dir - eg it something like xyz.ext
			// depending on ext - if the ext is for a page file
			// then generate OutDir/xyz/index.html
			// otherwise OutDir/xyz.ext
			ext := filepath.Ext(respath)

			rem := respath[:len(respath)-len(ext)]

			// TODO - also see if there is a .<lang> prefix on rem after ext has been removed
			// can use that for language sites
			destpath = filepath.Join(s.OutputDir, rem, "index.html")
		} else {
			// basic static file - so copy as is
			destpath = filepath.Join(s.OutputDir, respath)
		}
		destres := s.GetResource(destpath)
		destres.Source = r
		destres.Base = r.Base
		destres.frontMatter = r.frontMatter
		targets = append(targets, destres)
	}
	return
}

// LoadResource loads a resource and sets its basic properties, such as
// whether it's an index page or a parametric page.
func (h *BaseToHtmlRule) LoadResource(site *Site, r *Resource) error {
	// Other basic book keeping
	base := filepath.Base(r.FullPath)

	// check if it is an index page
	for _, ext := range h.Extensions {
		if r.IsIndex {
			break
		}
		for _, prefix := range []string{"index", "_index", "Index"} {
			if base == prefix+ext {
				r.IsIndex = true
				break
			}
		}
	}

	// check if it needs an index page - should this be only if we are NOT an index page?
	if slices.Contains(h.Extensions, r.Ext()) {
		r.NeedsIndex = true
	}

	base = filepath.Base(r.WithoutExt(true))
	r.IsParametric = base[0] == '[' && base[len(base)-1] == ']'

	// TODO - this needs to go - nothing magical about "Base"
	r.Site.CreateResourceBase(r)

	return nil
}
