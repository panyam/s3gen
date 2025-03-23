package s3gen

import (
	"bytes"
	"log"
	"path/filepath"
	"strings"
	"time"

	gotl "github.com/panyam/templar"
)

type HTMLResourceLoader struct {
}

func NewHTMLResourceLoader() *HTMLResourceLoader {
	return &HTMLResourceLoader{}
}

func (m *HTMLResourceLoader) Load(r *Resource) error {
	r.FrontMatter()
	if r.Error != nil {
		return r.Error
	}

	// Load the rest of the content so we can parse it
	source, err := r.ReadAll()
	if err != nil {
		return err
	}

	template := &gotl.Template{
		RawSource: source,
		Path:      r.FullPath,
		AsHtml:    true,
	}

	params := map[any]any{
		"Res":  r,
		"Site": r.Site,
		// "FrontMatter": res.FrontMatter().Data,
	}

	finalhtml := bytes.NewBufferString("")
	err = r.Site.Templates.RenderHtmlTemplate(finalhtml, template, "", params, nil)
	if err != nil {
		log.Println("Error loading html template content: ", err, r.FullPath)
		return err
	}
	// and parse it markdown content

	r.Document.Loaded = true
	r.Document.LoadedAt = time.Now()
	r.Document.Root = finalhtml.String()

	// Other bookkeeping
	base := filepath.Base(r.FullPath)
	r.IsIndex = base == "index.htm" || base == "_index.htm" || base == "index.html" || base == "_index.html"
	r.NeedsIndex = strings.HasSuffix(r.FullPath, ".htm") || strings.HasSuffix(r.FullPath, ".html")

	base = filepath.Base(r.WithoutExt(true))
	r.IsParametric = base[0] == '[' && base[len(base)-1] == ']'

	// TODO - this needs to go - nothing magical about "Page"
	r.Site.CreateResourceBase(r)
	return nil
}

type HTMLResourceRenderer struct {
	BaseResourceRenderer
}

func NewHTMLResourceRenderer() (out *HTMLResourceRenderer) {
	out = &HTMLResourceRenderer{}
	return out
}
