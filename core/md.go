package core

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	ttmpl "text/template"

	gut "github.com/panyam/goutils/utils"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

type MDContentProcessor struct {
	Template *ttmpl.Template
}

func NewMDContentProcessor(templatesDir string) *MDContentProcessor {
	h := &MDContentProcessor{}
	t := ttmpl.New("hello").Funcs(DefaultFuncMap())
	// Funcs(CustomFuncMap()).
	if t != nil && templatesDir != "" {
		var err error
		t, err = t.ParseGlob(templatesDir)
		if err != nil {
			log.Println("Error parsing templates: ", err)
		}
	}
	h.Template = t
	return h
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
	log.Println("FrontMatter: ", inres.FrontMatter())
	mdfile, _ := inres.Reader()
	defer mdfile.Close()

	mddata, _ := io.ReadAll(mdfile)
	outres.EnsureDir()

	mdTemplate, err := m.Template.Parse(string(mddata))
	if err != nil {
		log.Println("Template Parse Error: ", err)
		return err
	}

	finalmd := bytes.NewBufferString("")
	mdTemplate.Execute(finalmd, gut.StringMap{
		"Page": inres.FrontMatter,
	})

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
		),
	)
	var buf bytes.Buffer
	if err := md.Convert(finalmd.Bytes(), &buf); err != nil {
		log.Println("error converting md: ", err)
		return err
	}

	// How to load template?

	// we have the final "md" here - now we parse MD and arite *that* to outfile
	outfile, err := os.Create(outres.FullPath)
	if err != nil {
		log.Println("Error writing to: ", outres.FullPath, err)
		return err
	}
	log.Println("Writing to: ", outres.FullPath)
	defer outfile.Close()
	outfile.Write(buf.Bytes())
	return nil
}
