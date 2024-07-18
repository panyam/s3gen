package s3gen

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	gfn "github.com/panyam/goutils/fn"
)

type PageTemplate struct {
	Name   string
	Params map[any]any
}

// A page in our site.  These are what are finally rendered.
type DefaultPage struct {
	// Site this page belongs to - can this be in multiple - then create different
	// page instances
	Site *Site

	// The slug url for this page
	Slug string

	Title string

	Link string

	Summary string

	CreatedAt time.Time
	UpdatedAt time.Time

	IsDraft bool

	CanonicalUrl string

	Tags []string

	// The resource that corresponds to this page
	Res *Resource

	// The root view that corresponds to this page
	// By default - we use the BasePage view
	// RootView views.View[*Site]

	// Loaded, Pending, NotFound, Failed
	State int

	// Any errors with this resource
	Error error
}

func (page *DefaultPage) LoadFrom(res *Resource) error {
	frontMatter := res.FrontMatter().Data

	// For now we are going through "known" fields
	// TODO - just do this by dynamically going through all fields in FM
	// and calling setNestedProps and fail if this field doesnt exist - or using struct tags
	var err error
	if val, ok := frontMatter["tags"]; val != nil && ok {
		setNestedProp(page, gfn.Map(val.([]any), func(v any) string { return v.(string) }), "Tags")
	}
	if val, ok := frontMatter["title"]; val != nil && ok {
		page.Title = val.(string)
	}
	if val, ok := frontMatter["summary"]; val != nil && ok {
		page.Summary = val.(string)
	}
	if val, ok := frontMatter["date"]; val != nil && ok {
		// create at
		if val.(string) != "" {
			if page.CreatedAt, err = time.Parse("2006-1-2T03:04:05PM", val.(string)); err != nil {
				log.Println("error parsing created time: ", err)
			}
		}
	}

	if val, ok := frontMatter["lastmod"]; val != nil && ok {
		// update at
		if val.(string) != "" {
			if page.UpdatedAt, err = time.Parse("2006-1-2", val.(string)); err != nil {
				log.Println("error parsing last mod time: ", err)
			}
		}
	}

	if val, ok := frontMatter["draft"]; val != nil && ok {
		// update at
		page.IsDraft = val.(bool)
	}

	// see if we can calculate the slug and link urls
	site := page.Site
	page.Slug = ""
	relpath := ""
	resdir := res.DirName()
	if res.IsIndex {
		relpath, err = filepath.Rel(site.ContentRoot, resdir)
		if err != nil {
			return err
		}
	} else {
		fp := res.WithoutExt(true)
		relpath, err = filepath.Rel(site.ContentRoot, fp)
		if err != nil {
			return err
		}
	}
	if relpath == "." {
		relpath = ""
	}
	if relpath == "" {
		relpath = "/"
	}
	if relpath[0] == '/' {
		page.Link = fmt.Sprintf("%s%s", site.PathPrefix, relpath)
	} else {
		page.Link = fmt.Sprintf("%s/%s", site.PathPrefix, relpath)
	}

	/*
		pageName := "BasePage"
		if frontMatter["page"] != nil && frontMatter["page"] != "" {
			pageName = frontMatter["page"].(string)
		}
		page.RootView = site.NewView(pageName)
		if page.RootView == nil {
			log.Println("Could not find view: ", pageName)
		}
		page.RootView.SetPage(page)
	*/
	return nil
}
