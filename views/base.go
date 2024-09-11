package views

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type ViewRenderer interface {
	SetTemplate(templateName string)
	TemplateName() string
	RenderResponse(writer io.Writer) error
}

type ViewContainer[Context any] interface {
	ParentView() View[Context]
	AddChildViews(views ...View[Context])
	ChildViews() []View[Context]
}

type ViewPager interface {
	GetPage() any
	SetPage(any)
}

type View[Context any] interface {
	ViewPager
	ViewContainer[Context]
	ViewRenderer

	ViewId() string
	InitView(context Context, parentView View[Context])
	ValidateRequest(w http.ResponseWriter, r *http.Request) error
}

type BaseView[Context any] struct {
	Parent   View[Context]
	Id       string
	Context  Context
	Template string
	Children []View[Context]
	Page     any
	Loaded   bool
}

func (v *BaseView[C]) ViewId() string {
	return v.Id
}

func (v *BaseView[C]) SetTemplate(templateName string) {
	v.Template = templateName
}

func (v *BaseView[C]) TemplateName() string {
	return v.Template
}

func (v *BaseView[C]) GetPage() any {
	if v.Page == nil && v.Parent != nil {
		return v.Parent.GetPage()
	}
	return v.Page
}

func (v *BaseView[C]) SetPage(p any) {
	v.Page = p
	if v.Children != nil {
		for _, child := range v.Children {
			if child != nil {
				child.SetPage(p)
			}
		}
	}
}

func (v *BaseView[C]) InitView(s C, parent View[C]) {
	v.Context = s
	v.Parent = parent
	if v.Id == "" {
		v.Id = fmt.Sprintf("view_%d", time.Now().UnixMilli())
	}
}

// Sometimes a view may want to validate a request.
func (v *BaseView[C]) ValidateRequest(w http.ResponseWriter, r *http.Request) (err error) {
	for _, child := range v.Children {
		err = child.ValidateRequest(w, r)
		if err != nil {
			return
		}
	}
	return
}

func (v *BaseView[C]) RenderResponse(writer io.Writer) (err error) {
	return nil
}

func (v *BaseView[C]) AddChildViews(views ...View[C]) {
	for idx, child := range views {
		if child != nil {
			v.Children = append(v.Children, child)
			// we can add nil children as views to reserve spots
			child.InitView(v.Context, v)
		} else {
			log.Println("Child is nil, idx: ", idx)
		}
	}
}

func (v *BaseView[C]) ParentView() View[C] {
	return v.Parent
}

func (v *BaseView[C]) ChildViews() []View[C] {
	return v.Children
}
