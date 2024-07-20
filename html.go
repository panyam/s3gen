package s3gen

import (
	"bytes"
	htmpl "html/template"
	"io"
	"log"
	"log/slog"
	"path/filepath"
	"strings"
)

type HTMLResourceHandler struct {
	defaultResourceHandler
	Template *htmpl.Template
}

func NewHTMLResourceHandler(templatesDir string) *HTMLResourceHandler {
	h := &HTMLResourceHandler{}
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

func (m *HTMLResourceHandler) LoadResource(res *Resource) error {
	// Make sure resource's front matter is loaded if any
	res.FrontMatter()

	base := filepath.Base(res.FullPath)
	res.IsIndex = base == "index.htm" || base == "_index.htm" || base == "index.html" || base == "_index.html"
	res.NeedsIndex = strings.HasSuffix(res.FullPath, ".htm") || strings.HasSuffix(res.FullPath, ".html")

	base = filepath.Base(res.WithoutExt(true))
	res.IsParametric = base[0] == '[' && base[len(base)-1] == ']'

	// if we are not parametric - then created the destination page
	res.Site.CreatePage(res)
	// res.Page.LoadFrom(res)
	return nil
}

func (m *HTMLResourceHandler) LoadParamValues(res *Resource) error {
	output := bytes.NewBufferString("")
	err := m.RenderContent(res, output)
	if err != nil {
		log.Println("Error executing paramvals template: ", err, res.FullPath)
	} else {
		slog.Debug("Param Values After: ", "pvals", res.ParamValues, "content", output)
	}
	return err
}

func (m *HTMLResourceHandler) RenderContent(res *Resource, w io.Writer) error {
	site := res.Site
	mdfile, _ := res.Reader()
	mddata, _ := io.ReadAll(mdfile)
	defer mdfile.Close()

	mdTemplate, err := site.HtmlTemplate(true).Parse(string(mddata))
	if err != nil {
		slog.Error("Template Clone Error: ", "error", err)
		return err
	}
	if err != nil {
		mdTemplate, err = mdTemplate.Parse(string(mddata))
	}
	if err != nil {
		return err
	}

	params := map[any]any{
		"Res":         res,
		"Site":        res.Site,
		"FrontMatter": res.FrontMatter().Data,
	}
	err = mdTemplate.Execute(w, params)
	if err != nil {
		log.Println("Error executing HTML: ", res.FullPath, err)
	}
	return err
}
