package core

import (
	"log"
	"path/filepath"
	"strings"
	htmpl "text/template"
)

type HTMLContentProcessor struct {
	Template *htmpl.Template
}

func NewHTMLContentProcessor(templatesDir string) *HTMLContentProcessor {
	h := &HTMLContentProcessor{}
	t, err := htmpl.New("hello").
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
