package s3gen

import (
	"bytes"
	"io"
	"log"
	"log/slog"
	"path/filepath"
	"strings"

	gotl "github.com/panyam/templar"
)

type HTMLResourceHandler struct {
	defaultResourceHandler
}

func NewHTMLResourceHandler(templatesDir string) *HTMLResourceHandler {
	h := &HTMLResourceHandler{}
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

	template := &gotl.Template{
		RawSource: mddata,
		Path:      res.FullPath,
		AsHtml:    true,
	}

	params := map[any]any{
		"Res":         res,
		"Site":        res.Site,
		"FrontMatter": res.FrontMatter().Data,
	}
	return site.Templates.RenderHtmlTemplate(w, template, params, nil)
}
