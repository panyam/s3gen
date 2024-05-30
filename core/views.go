package core

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"

	"github.com/morrisxyang/xreflect"
)

type View interface {
	InitView(site *Site, parentView View)
	ValidateRequest(w http.ResponseWriter, r *http.Request) error
	RenderResponse(writer io.Writer) (err error)
	TemplateName() string
	ParentView() View
	AddChildViews(views ...View)
	ChildViews() []View
	SelfView() View
	GetPage() any
	SetPage(any)
}

func SetViewProp(obj any, value any, fieldpath string) error {
	return xreflect.SetEmbedField(obj, fieldpath, value)
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
}

func GetViewProp(view View, fieldpath ...any) any {
	return nil
}

type BaseView struct {
	Parent   View
	Site     *Site
	Template string
	Children []View
	Self     View
	Page     any
}

func (v *BaseView) SelfView() View {
	return v.Self
}

func (v *BaseView) ParentView() View {
	return v.Parent
}

func (v *BaseView) ChildViews() []View {
	return v.Children
}

func (v *BaseView) TemplateName() string {
	return v.Template
}

func (v *BaseView) GetPage() any {
	if v.Page == nil && v.Parent != nil {
		return v.Parent.GetPage()
	}
	return v.Page
}

func (v *BaseView) SetPage(p any) {
	v.Page = p
	if v.Children != nil {
		for _, child := range v.Children {
			if child != nil {
				child.SetPage(p)
			}
		}
	}
}

func (v *BaseView) InitView(s *Site, parent View) {
	v.Site = s
	v.Parent = parent
	if v.Children != nil {
		for _, child := range v.Children {
			if child == nil {
				// log.Println("Child is nil, idx: ", idx)
			} else {
				child.InitView(s, v)
			}
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
	// log.Println("T: ", t, t.Kind())
	// log.Println("E: ", e, "Kind: ", e.Kind(), "Name: ", e.Name(), "PkgPath: ", e.PkgPath())
	if v.TemplateName() == "" {
		t := reflect.TypeOf(v.Self)
		e := t.Elem()
		// use the type here
		err := v.Site.HtmlTemplate().ExecuteTemplate(writer, e.Name(), v.Self)
		if err != nil {
			log.Println("Error with e.Name(), Error: ", e.Name(), err)
			// try with the .html name
			err = v.Site.HtmlTemplate().ExecuteTemplate(writer, e.Name()+".html", v.Self)
		}
		if err != nil {
			log.Println("Error with e.Name().html, Error: ", e.Name(), err)
			_, err = writer.Write([]byte(fmt.Sprintf("Template error: %s", err.Error())))
		}
	} else {
		return v.Site.HtmlTemplate().ExecuteTemplate(writer, v.TemplateName(), v.Self)
	}
	return
}
