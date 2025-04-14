package s3gen

import (
	"log"
	"path/filepath"
	"strings"
)

// BaseTemplates are data used to render a page.  Typically this needs the name of the template
// being rendered and the (possibly nested) parameters need by that specific template.
type BaseTemplate struct {
	// Name of the Template file that is to be used as the root
	Name string

	// Name of the template within the template file for the entry point (given a
	// template file may contain multiple templates)
	Entry string

	Params map[any]any
}

// We are switching to a rule based build system.
// Problem with a Resource having a loader and its output resource having a "renderer" is the model doesnt quite make
// sense.  The renderer should be on the input resource and the output is just an artifact.  And a renderer really
// depends not just on the output resource but on both input and output.  ie we have some kind of multi-arg dispatch
// instead of based on a single entity (either in or out resource)
//
// A Rule based approach makes this simpler and more generic.
//
// In the case of a single .md file being rendered as a single .html file (eg /contentroot/a.md -> /output/a/index.html)
// it is straightforward - load a.md, parse a.md, find a base template, render the parsed AST onto the output file.
//
// This is great for 1:1 files.  However any change in arity (in either side) breaks this.  Say we had:
// 1. a "folder" with multiple parts, that forms a combined output file or
// 2. a single .md file that is broken into multiple parts (each slides, or sections etc)
// 3. or many to many - a bundle that generates another bundle.
//
// Here it is very hard to associate a rendered to any one of the resources.  Worse even the loader is not constant,
// depending on the build action a different loader (eg parser) may be needed.
//
// So what we need is a way to associate resource to one or more build rules that can process the resources in generating
// the right artificats.

type Rule interface {
	// Given an input resource, finds all sibling and targets resources that will be affected by it For example if we have
	// a directory with X files.  This can result in an output bundle of Y files/resources.   This method when given a
	// file/res collects all "related" or "sibling" input resources needed to generate all targets that depend on them.
	//
	// This has a few benefits.   By grouping related/sibling resources, the site itself does not have revisit
	// a sibling resource for the same rule (note a resource can be applied by multiple rules)
	TargetsFor(site *Site, res *Resource) (siblings []*Resource, targets []*Resource)

	// Generate the output resource for a related set of "input" targets
	Run(site *Site, inputs []*Resource, targets []*Resource, funcs map[string]any) error
}

type BaseToHtmlRule struct {
	Extensions []string
}

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

// Given an input resource, finds all sibling and targets resources that will be affected by it For example if we have
// a directory with X files.  This can result in an output bundle of Y files/resources.   This method when given a
// file/res collects all "related" or "sibling" input resources needed to generate all targets that depend on them.
//
// This has a few benefits.   By grouping related/sibling resources, the site itself does not have revisit
// a sibling resource for the same rule (note a resource can be applied by multiple rules)
func (m *BaseToHtmlRule) TargetsFor(s *Site, r *Resource) (siblings []*Resource, targets []*Resource) {
	respath, found := strings.CutPrefix(r.FullPath, s.ContentRoot)
	if !found {
		log.Println("Respath not found: ", r.FullPath, s.ContentRoot)
		return nil, nil
	}

	isValidExt := false
	for _, ext := range m.Extensions {
		if r.Ext() == ext {
			isValidExt = true
			break
		}
	}
	if !isValidExt {
		return
	}

	log.Println("isValid, Res, Extensions, isParametric, isIndex, needsIndex: ", isValidExt, r.FullPath, m.Extensions, r.IsParametric, r.IsIndex, r.NeedsIndex)
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
	for _, ext := range h.Extensions {
		if r.Ext() == ext {
			r.NeedsIndex = true
			break
		}
	}

	base = filepath.Base(r.WithoutExt(true))
	r.IsParametric = base[0] == '[' && base[len(base)-1] == ']'

	// TODO - this needs to go - nothing magical about "Page"
	r.Site.CreateResourceBase(r)

	return nil
}
