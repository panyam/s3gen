package core

import (
	"bytes"
	"io"
	"log"
	"net/http"

	"github.com/morrisxyang/xreflect"
	gut "github.com/panyam/goutils/utils"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

type View interface {
	InitContext(*Site, View)
	ValidateRequest(w http.ResponseWriter, r *http.Request) error
	RenderResponse(writer io.Writer) (err error)
	TemplateName() string
}

func SetViewProp(obj any, value any, fieldpath string) error {
	xreflect.SetEmbedField(obj, fieldpath, value)
	/*
			var prev reflect.Value
			curr := reflect.ValueOf(view)
			for i, fpart := range fieldpath {
				prev = curr
				currtype := curr.Type()
				currkind := curr.Kind()
				log.Println("Type: ", currtype, currtype.Name(), currkind)
				if reflect.TypeOf(fpart).String() == "string" {
					if currkind == reflect.Ptr {
						curr = curr.Elem()
					}
					curr = curr.FieldByName(fpart.(string))
				} else {
					curr = curr.Index(fpart.(int))
				}
				if !curr.IsValid() {
					log.Println("Is Valid is false: ", i, len(fieldpath))
					if i == len(fieldpath)-1 {
						// At the end so we can set this
					}
				}
			}
		log.Println("end of loop, curr: ", prev, curr, view, fieldpath)
	*/
	return nil
}

func GetViewProp(view View, fieldpath ...any) any {
	return nil
}

type BaseView struct {
	Parent   View
	Site     *Site
	Template string
	Children []View
}

func (v *BaseView) TemplateName() string {
	return v.Template
}

func (v *BaseView) InitContext(s *Site, parent View) {
	v.Site = s
	v.Parent = parent
	if v.Children != nil {
		for _, child := range v.Children {
			child.InitContext(s, v)
		}
	}
}

func (v *BaseView) ValidateRequest(w http.ResponseWriter, r *http.Request) (err error) {
	for _, child := range v.Children {
		err = child.ValidateRequest(w, r)
		if err != nil {
			return
		}
	}
	return
}

func (v *BaseView) AddChildViews(views ...View) {
	for _, child := range views {
		v.Children = append(v.Children, child)
	}
}

func (v *BaseView) RenderResponse(writer io.Writer) (err error) {
	if v.Template == "" {
		_, err = writer.Write([]byte("TemplateName not provided"))
	} else {
		return v.Site.HtmlTemplate.ExecuteTemplate(writer, v.Template, v)
	}
	return
}

// A view that renders a Markdown
type MDView struct {
	BaseView

	// Actual resource to render
	Res *Resource
}

func (v *MDView) RenderResponse(writer io.Writer) (err error) {
	res := v.Res
	mdfile, _ := res.Reader()
	mddata, _ := io.ReadAll(mdfile)
	defer mdfile.Close()

	mdTemplate, err := v.Site.TextTemplate.Parse(string(mddata))
	if err != nil {
		log.Println("Template Parse Error: ", err)
		return err
	}

	finalmd := bytes.NewBufferString("")
	err = mdTemplate.Execute(finalmd, gut.StringMap{
		"Site": v.Site,
		"Page": res.FrontMatter,
	})
	if err != nil {
		log.Println("Error executing MD: ", err)
	}

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

	// Now load the layout
	frontMatter := res.FrontMatter()
	layoutName := ""
	if frontMatter != nil && frontMatter.Data != nil {
		if frontMatter.Data["layout"] != "" {
			if layout, ok := frontMatter.Data["layout"].(string); ok {
				layoutName = layout
			}
		}
	}
	if layoutName == "" { // If we dont have a layout name then render as is
		writer.Write(buf.Bytes())
	} else {
		// out2 := bytes.NewBufferString("")
		err = v.Site.HtmlTemplate.ExecuteTemplate(writer, layoutName, map[string]any{
			"Content":      string(buf.Bytes()),
			"SiteMetadata": v.Site.SiteMetadata,
			"PrevPost":     nil,
			"NextPost":     nil,
			"Post": map[string]any{
				"Slug": "testslug",
			},
		})
	}
	return err
}
