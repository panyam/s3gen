package core

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrg/frontmatter"
	gfn "github.com/panyam/goutils/fn"
)

const (
	ResourceStatePending = iota
	ResourceStateLoaded
	ResourceStateDeleted
	ResourceStateNotFound
	ResourceStateFailed
)

type ResourceFilterFunc func(res *Resource) bool
type ResourceSortFunc func(a *Resource, b *Resource) bool
type PageFilterFunc func(res *Page) bool
type PageSortFunc func(a *Page, b *Page) bool

type FrontMatter struct {
	Loaded bool
	Data   map[string]any
	Length int64
}

// Our interface for returning all static content in our site
type ResourceService interface {
	GetResource(fullpath string) *Resource
	ListResources(filterFunc func(res *Resource) bool,
		sortFunc func(a *Resource, b *Resource) bool,
		offset int, count int) []*Resource
	ListPages(filterFunc func(res *Page) bool,
		sortFunc func(a *Page, b *Page) bool,
		offset int, count int) []*Page
}

// A page in our site.  These are what are finally rendered.
type Page struct {
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
	// TODO - Should this be just the root resource or all resources for it?
	Res *Resource
	// DestRes *Resource

	// Tells whether this is a detail page or a listing page
	IsListPage bool

	// The root view that corresponds to this page
	// By default - we use the BasePage view
	RootView View

	// Loaded, Pending, NotFound, Failed
	State int

	// Any errors with this resource
	Error error
}

func (page *Page) LoadFrom(res *Resource) error {
	frontMatter := res.FrontMatter().Data
	pageName := "BasePage"
	if frontMatter["page"] != nil && frontMatter["page"] != "" {
		pageName = frontMatter["page"].(string)
	}
	site := page.Site
	page.RootView = site.NewView(pageName)
	if page.RootView == nil {
		log.Println("Could not find view: ", pageName)
	}
	page.RootView.SetPage(page)

	// For now we are going through "known" fields
	// TODO - just do this by dynamically going through all fields in FM
	// and calling SetViewProps and fail if this field doesnt exist - or using struct tags
	var err error
	if val, ok := frontMatter["tags"]; val != nil && ok {
		SetViewProp(page, gfn.Map(val.([]any), func(v any) string { return v.(string) }), "Tags")
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
	return nil
}

/**
 * Each resource in our static site is identified by a unique path.
 * Note that resources can go through multiple transformations
 * resulting in more resources - to be converted into other resources.
 * Each resource is uniquely identified by its full path
 */
type Resource struct {
	Site     *Site
	FullPath string // Unique URI/Path

	// Created timestamp on disk
	CreatedAt time.Time

	// Updated time stamp on disk
	UpdatedAt time.Time

	// Loaded, Pending, NotFound, Failed
	State int

	// Any errors with this resource
	Error error

	// Info about the resource
	info os.FileInfo

	IsIndex      bool
	NeedsIndex   bool
	IsParametric bool

	// Marks whether front matter was loaded
	frontMatter FrontMatter

	// The destination page if this resource is for a target page
	DestPage *Page

	// If this is a parametric resources - this returns the space of all parameters
	// possible for this resource based on how it is loaded and its config it takes
	// For example a blog page of the form /a/b/c/[name].md could have 10 distinct values
	// for the "name" parameter.  Those will be populated here by the content processor
	ParamValues      []string
	CurrentParamName string

	// Once ParamValues are captured, the site will render this render this resource
	// once per Param value.   Each page will be rendered in a different location.
	ParamPages map[string]*Page
}

func (r *Resource) AddParam(param string) *Resource {
	r.ParamValues = append(r.ParamValues, param)
	return r
}

func (r *Resource) AddParams(params []string) *Resource {
	for _, param := range params {
		r.ParamValues = append(r.ParamValues, param)
	}
	return r
}

func (r *Resource) LoadParamValues() {
	s := r.Site
	proc := s.GetResourceLoader(r)
	if proc != nil && r.State == ResourceStateLoaded {
		r.ParamValues = nil
		proc.LoadParamValues(r)
	}
}

func (r *Resource) Load() *Resource {
	s := r.Site
	proc := s.GetResourceLoader(r)
	if proc != nil && r.State == ResourceStatePending {
		r.Error = proc.LoadResource(r)
		if r.Error != nil {
			log.Println("Error loading rource: ", r.Error, r.FullPath)
		} else {
			r.State = ResourceStateLoaded
		}
	}
	return r
}

func (r *Resource) Reset() {
	r.State = ResourceStatePending
	r.info = nil
	r.Error = nil
	r.frontMatter.Loaded = false
	r.DestPage = nil
	r.ParamValues = nil
	r.ParamPages = make(map[string]*Page)
}

// Ensures that a resource's parent directory exists
func (r *Resource) EnsureDir() {
	dirname := filepath.Dir(r.FullPath)
	if err := os.MkdirAll(dirname, 0755); err != nil {
		log.Println("Error creating dir: ", dirname, err)
	}
}

// Returns the resource's directory
func (r *Resource) DirName() string {
	return filepath.Dir(r.FullPath)
}

// Returns the resource without the extension.
func (r *Resource) WithoutExt(all bool) string {
	out := r.FullPath
	for {
		ext := filepath.Ext(out)
		if ext == "" {
			break
		} else {
			out = out[:len(out)-len(ext)]
			if !all {
				break
			}
		}
	}
	return out
}

func (r *Resource) Info() os.FileInfo {
	if r.info == nil {
		r.info, r.Error = os.Stat(r.FullPath)
		if r.Error != nil {
			r.State = ResourceStateFailed
			log.Println("Error Getting Info: ", r.FullPath, r.Error)
		}
	}
	return r.info
}

// Read all the content bytes after the front-matter in this file.
func (r *Resource) ReadAll() ([]byte, error) {
	reader, err := r.Reader()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

// Loads the front matter for a resource if it exists
func (r *Resource) Reader() (io.ReadCloser, error) {
	// Read the content
	r.FrontMatter()

	fi, err := os.Open(r.FullPath)
	if err != nil {
		return nil, err
	}
	fi.Seek(r.frontMatter.Length, 0)
	return fi, nil
}

func (r *Resource) IsDir() bool {
	return r.Info().IsDir()
}

func (r *Resource) Ext() string {
	return filepath.Ext(r.FullPath)
}

func (r *Resource) FrontMatter() *FrontMatter {
	if !r.frontMatter.Loaded {
		f, err := os.Open(r.FullPath)
		if err != nil {
			r.Error = err
			r.State = ResourceStateFailed
		} else {
			r.frontMatter.Data = make(map[string]any)
			// TODO: We want a library that just returns frontMatter and Length
			// this way we dont need to load the entire content unless we needed
			// and even then we could just do it via a reader
			rest, err := frontmatter.Parse(f, r.frontMatter.Data)
			r.frontMatter.Length = r.Info().Size() - int64(len(rest))
			if err != nil {
				r.Error = err
				r.State = ResourceStateFailed
			} else {
				r.frontMatter.Loaded = true
			}
		}
	}
	return &r.frontMatter
}

func (r *Resource) PageFor(param string) *Page {
	if param == "" {
		return r.DestPage
	}
	if r.ParamPages == nil {
		r.ParamPages = make(map[string]*Page)
	}
	page, ok := r.ParamPages[param]
	if !ok || page == nil {
		page = &Page{Site: r.Site, Res: r}
		r.ParamPages[param] = page
		page.LoadFrom(r)
	}
	return page
}

// Returns the path relative to the content root
func (r *Resource) RelPath() string {
	respath, found := strings.CutPrefix(r.FullPath, r.Site.ContentRoot)
	if !found {
		return ""
	}
	return respath
}

func (r *Resource) DestPathFor(param string) (destpath string) {
	s := r.Site
	respath, found := strings.CutPrefix(r.FullPath, s.ContentRoot)
	if !found {
		log.Println("Respath not found: ", r.FullPath, s.ContentRoot)
		return ""
	}

	if r.IsParametric {
		if param == "" {
			log.Println("Page is parametric but param is empty: ", r.FullPath)
			return
		}

		// if we have /a/b/c/d/[param].ext
		// then do /a/b/c/d/param/index.html
		// res is not a dir - eg it something like xyz.ext
		// depending on ext - if the ext is for a page file
		// then generate OutDir/xyz/index.html
		// otherwise OutDir/xyz.ext
		ext := filepath.Ext(respath)

		rem := respath[:len(respath)-len(ext)]
		dirname := filepath.Dir(rem)

		// TODO - also see if there is a .<lang> prefix on rem after ext has been removed
		// can use that for language sites
		destpath = filepath.Join(s.OutputDir, dirname, param, "index.html")
	} else {
		if r.Info().IsDir() {
			// Then this will be served with dest/index.html
			destpath = filepath.Join(s.OutputDir, respath)
		} else if r.IsIndex {
			destpath = filepath.Join(s.OutputDir, filepath.Dir(respath), "index.html")
		} else if r.NeedsIndex {
			// res is not a dir - eg it something like xyz.ext
			// depending on ext - if the ext is for a page file
			// then generate OutDir/xyz/index.html
			// otherwise OutDir/xyz.ext
			ext := filepath.Ext(respath)

			rem := respath[:len(respath)-len(ext)]

			// TODO - also see if there is a .<lang> prefix on rem after ext has been removed
			// can use that for language sites
			destpath = filepath.Join(s.OutputDir, rem, "index.html")
		} else {
			// basic static file - so copy as is
			destpath = filepath.Join(s.OutputDir, respath)
		}
	}
	return
}
