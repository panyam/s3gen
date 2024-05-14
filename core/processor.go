package core

import (
	"html/template"
	"log"
	"path/filepath"
	"strings"
)

type ContentProcessor interface {
	// Called to handle a resource -
	// This can generate more resources
	IsIndex(s *Site, res *Resource) bool
	NeedsIndex(s *Site, res *Resource) bool
	Process(s *Site, inres *Resource, outres *Resource) error
}

func (s *Site) DestResourceFor(res *Resource) *Resource {
	// if a resource is in the content root - then return its "equiv" in the output
	// This also ensures that we have problem "Foo/index.html" for Foo.md files
	respath, found := strings.CutPrefix(res.FullPath, s.ContentRoot)
	if !found {
		log.Println("H1: ", res.FullPath, s.ContentRoot)
		return nil
	}

	if res.Info() == nil {
		log.Println("here 2....")
		return nil
	}

	proc := s.GetContentProcessor(res)
	destpath := ""
	if res.Info().IsDir() {
		// Then this will be served with dest/index.html
		destpath = filepath.Join(s.OutputDir, respath)
	} else if proc.IsIndex(s, res) {
		destpath = filepath.Join(s.OutputDir, filepath.Dir(respath), "index.html")
	} else if proc.NeedsIndex(s, res) {
		// res is not a dir - eg it something like xyz.ext
		// depending on ext - if the ext is for a page file
		// then generate OutDir/xyz/index.html
		// otherwise OutDir/xyz.ext
		ext := filepath.Ext(respath)

		rem := respath[:len(respath)-len(ext)]

		// TODO - also see if there is a .<lang> prefix on rem after ext has been removed
		// can use that for language sites
		destpath = filepath.Join(rem, "index.html")
		log.Println("ResP: ", respath, "Remaining: ", rem)
	} else {
		// basic static file - so copy as is
		destpath = filepath.Join(s.OutputDir, respath)
	}
	// log.Println("Res, Dest Paths: ", respath, destpath)
	return s.GetResource(destpath)
}

func (s *Site) GetContentProcessor(rs *Resource) ContentProcessor {
	// normal file
	// check type and call appropriate processor
	// Should we call processor directly here or collect a list and
	// pass that to Rebuild with those resources?
	ext := filepath.Ext(rs.FullPath)

	// TODO - move to a table lookup or regex based one
	if ext == ".mdx" || ext == ".md" {
		return &MDContentProcessor{}
	}
	if ext == ".html" || ext == ".htm" {
		return &HTMLContentProcessor{}
	}
	log.Println("Could not find proc for, Name, Ext: ", rs.FullPath, ext)
	return nil
}

type MDContentProcessor struct {
}

// Returns a list of output resources that depend on this resource
func (m *MDContentProcessor) NeedsIndex(s *Site, res *Resource) bool {
	return strings.HasSuffix(res.FullPath, ".md") || strings.HasSuffix(res.FullPath, ".mdx")
}

func (m *MDContentProcessor) IsIndex(s *Site, res *Resource) bool {
	base := filepath.Base(res.FullPath)
	return base == "index.md" || base == "_index.md" || base == "index.mdx" || base == "_index.mdx"
}

func (m *MDContentProcessor) Process(s *Site, inres *Resource, outres *Resource) error {
	log.Println("MD Processing: ", inres.FullPath, "------>", outres.FullPath)
	return nil
}

type HTMLContentProcessor struct {
	Template *template.Template
}

func NewHTMLContentProcessor(templatesDir string) *HTMLContentProcessor {
	h := &HTMLContentProcessor{}
	t, err := template.New("hello").
		Funcs(DefaultFuncMap()).
		// Funcs(CustomFuncMap()).
		ParseGlob(templatesDir)
	if err != nil {
		panic(err)
	}
	h.Template = t
	return h
}

func (h *HTMLContentProcessor) Process(s *Site, inres *Resource, outres *Resource) error {
	// 1. Load the res file
	// 2. find target res (and output dir)
	// 3. render it to target file
	// 4. return target res
	log.Println("HTML Processing: ", inres.FullPath, "------>", outres.FullPath)
	return nil
}

func (m *HTMLContentProcessor) IsIndex(s *Site, res *Resource) bool {
	base := filepath.Base(res.FullPath)
	return base == "index.htm" || base == "_index.htm" || base == "index.html" || base == "_index.html"
}

func (m *HTMLContentProcessor) NeedsIndex(s *Site, res *Resource) bool {
	return strings.HasSuffix(res.FullPath, ".htm") || strings.HasSuffix(res.FullPath, ".html")
}
