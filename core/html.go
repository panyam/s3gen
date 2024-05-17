package core

import (
	htmpl "html/template"
	"io"
	"log"
	"path/filepath"
	"strings"
)

type HTMLContentProcessor struct {
	Template *htmpl.Template
}

func NewHTMLContentProcessor(templatesDir string) *HTMLContentProcessor {
	h := &HTMLContentProcessor{}
	h.Template = htmpl.New("hello")
	if templatesDir != "" {
		// Funcs(CustomFuncMap()).
		t, err := h.Template.ParseGlob(templatesDir)
		if err != nil {
			log.Println("Error loading dir: ", templatesDir, err)
		} else {
			h.Template = t
		}
	}
	return h
}

func (h *HTMLContentProcessor) Process(s *Site, inres *Resource, writer io.Writer) error {
	// 1. Load the res file
	// 2. find target res (and output dir)
	// 3. render it to target file
	// 4. return target res
	return nil
}

func (m *HTMLContentProcessor) IsIndex(s *Site, res *Resource) bool {
	base := filepath.Base(res.FullPath)
	return base == "index.htm" || base == "_index.htm" || base == "index.html" || base == "_index.html"
}

func (m *HTMLContentProcessor) NeedsIndex(s *Site, res *Resource) bool {
	return strings.HasSuffix(res.FullPath, ".htm") || strings.HasSuffix(res.FullPath, ".html")
}

// Idea is the resource may have a lot of information on how it should be rendered
// Given a page we want to identify what page properties should be set from here
// so when page is finally rendered it is all uptodate
func (h *HTMLContentProcessor) PopulatePage(res *Resource, page *Page) error {
	return nil
}
