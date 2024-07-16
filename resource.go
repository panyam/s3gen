package s3gen

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrg/frontmatter"
)

type Page interface {
	LoadFrom(*Resource) error
}

const (
	// When a resource is first encountered it is in pending state to indicate it needs to be loaded
	ResourceStatePending = iota

	// Marked when a resource has been loaded without any errors.
	ResourceStateLoaded

	// When a previously loaded resource has been deleted.
	ResourceStateDeleted

	// To indicate that a resource is not found (for some reason)
	ResourceStateNotFound

	// Loading of a resource failed (the status will be in the error field)
	ResourceStateFailed
)

// All files in our site are represented by the Resource type.
// Each resource in identified by a unique path.   A resource can be processed
// or transformed to result in more Resources (eg an input post markdown resource
// would be rendered as an output html resource).
type Resource struct {
	// The Site that owns this resources.  Resources belong to a site and cannot be shared across multiple sites
	Site *Site

	// Fullpath of the Resource uniquely identifying it within a Site
	FullPath string

	// Created timestamp on disk
	CreatedAt time.Time

	// Updated time stamp on disk
	UpdatedAt time.Time

	// The ResourceState - Loaded, Pending, NotFound, Failed
	State int

	// Any errors with this resource (eg during load)
	Error error

	// os level Info about the resource
	info os.FileInfo

	IsIndex      bool
	NeedsIndex   bool
	IsParametric bool

	// Marks whether front matter was loaded
	frontMatter FrontMatter

	// The destination page if this resource is for a target page
	DestPage Page

	// If this is a parametric resources - this returns the space of all parameters
	// possible for this resource based on how it is loaded and its config it takes
	// For example a blog page of the form /a/b/c/[name].md could have 10 distinct values
	// for the "name" parameter.  Those will be populated here by the content processor
	// For now we are only looking at single level of parameters.  In the future we will
	// consider multiple parameters, eg: /[param1]/[param2]...
	TemplateResource *Resource
	ParamValues      []string
	CurrentParamName string

	// Once ParamValues are captured, the site will render this render this resource
	// once per Param value.   Each page will be rendered in a different location.
	ParamResources map[string]*Resource
}

// Load's the resource from disk including any front matter it might have.
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

// Reset's the Resource's state to Pending so it can be reloaded
func (r *Resource) Reset() {
	r.State = ResourceStatePending
	r.info = nil
	r.Error = nil
	r.frontMatter.Loaded = false
	r.DestPage = nil
	r.ParamValues = nil
	r.ParamResources = make(map[string]Page)
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

// Get the resource's os level FileInfo
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

// Returns a reader for all the content bytes after the front matter for a resource if it exists.
// This is handy to prevent loading entire large files into memory (unlike ReadAll).
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

// Returns true if the resource is a directory
func (r *Resource) IsDir() bool {
	return r.Info().IsDir()
}

// Returns the extension of the resource's path
func (r *Resource) Ext() string {
	return filepath.Ext(r.FullPath)
}

// Load's the resource's front matter and parses if it has not been done so before.
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

func (r *Resource) PageFor(param string) *Resource {
	if param == "" {
		return r
	}
	if r.ParamResources == nil {
		r.ParamResources = make(map[string]*Resource)
	}
	page, ok := r.ParamResources[param]
	if !ok || page == nil {
		page = &DefaultPage{Site: r.Site, Res: r}
		r.ParamResources[param] = page
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

// Loads a resource of diferent types from storage
type ResourceLoader interface {
	// Loads resource data from the appropriate input path
	LoadResource(r *Resource) error

	// Loads the parameter values for a resource
	// This is seperate from the resource as this is called only for
	// a paraametric page.  Typically parametric pages will need to know
	// about everything else in the site so the site and its (leaf) resource
	// has to be loaded before this is called.  Hence it is seperated from
	// the normal (leaf) load of a resource.  If the load is successful
	// thenthe r.ParamValues is set to all the parametrics this page can take
	// otherwise an error is returned
	LoadParamValues(r *Resource) error

	// Sets up the view for a page
	SetupPageView(res *Resource, page Page) (err error)
}

type ResourceFilterFunc func(res *Resource) bool
type ResourceSortFunc func(a *Resource, b *Resource) bool

// Each Resource may have front matter.  Front matter is lazily loaded and parsed in a resource.
// Our Resources specifically keep a reference to the front matter which can be used later on
// during rendering
type FrontMatter struct {
	// Whether the front matter for the resource has been loaded or not
	Loaded bool

	// Parsed data from front matter
	Data map[string]any

	// Length of the frontmatter in bytes (will be set after it is loaded)
	Length int64
}
