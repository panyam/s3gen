package core

import (
	"html/template"
	"io"
	"log"
	"net/http"
)

type Context struct {
	Writer   io.Writer
	Template *template.Template
}

func (c *Context) Render(v View, templateName string) error {
	if templateName != "" {
		return c.Template.ExecuteTemplate(c.Writer, templateName, v)
	}

	templateName = v.TemplateName()
	if templateName != "" {
		return c.Template.ExecuteTemplate(c.Writer, templateName, v)
	}
	return v.RenderResponse()
}

type View interface {
	InitContext(*Context, View)
	ValidateRequest(w http.ResponseWriter, r *http.Request) error
	RenderResponse() (err error)
	TemplateName() string
}

type BaseView struct {
	Parent   View
	Ctx      *Context
	Template string
	Children []View
}

func (v *BaseView) TemplateName() string {
	return v.Template
}

func (v *BaseView) InitContext(c *Context, parent View) {
	v.Ctx = c
	v.Parent = parent
	for _, child := range v.Children {
		log.Println("this, child: ", v, child)
		child.InitContext(c, v)
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

func (v *BaseView) RenderResponse() (err error) {
	if v.Template == "" {
		_, err = v.Ctx.Writer.Write([]byte("TemplateName not provided"))
	} else {
		return v.Ctx.Template.ExecuteTemplate(v.Ctx.Writer, v.Template, v)
	}
	return
}
