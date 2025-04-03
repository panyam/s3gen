package s3gen

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

// Instead of a single "handler" what we have just stages a resources goes through
// Read -> Parse -> Generate Targets (other resources) -> Render Resources

// Loads and parses a resource.  This ensures that a resource's Document will now return a valid value.
type ResourceLoader interface {
	Load(res *Resource) error
}

type ResourceProcessor interface {
	GenerateTargets(res *Resource, deps map[string]*Resource) error
}

// Render's a resource onto an output stream.
type ResourceRenderer interface {
	Render(res *Resource, w io.Writer) error
}

type ResourceBase interface {
	LoadFrom(*Resource) error
}

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

// Each Resource can be parsed into a document.  The document is the AST that can be transformed
// by otherways by various callers to annotate what ever info they need.  eg some targets may
// want to build a TOC out of a parsed Markdown.  So just haveint he MD in parsed form lets us
// do all kinds of transformations.  Where as others may want to filter all sections into
// some form etc.
type Document struct {
	// Whether the document has been loaded and parsed
	Loaded bool

	// When the document was loaded (if at all)
	LoadedAt time.Time

	// Root of the parsed document tree
	Root any

	// This is a way to store extra MD about a document after it has been parsed.
	// eg this would be a place to store a document's TOC if/when needed by some downstream
	Metadata map[string]any
}

func (d *Document) SetMetadata(k string, v any) {
	if d.Metadata == nil {
		d.Metadata = map[string]any{}
	}
	d.Metadata[k] = v
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
	Base ResourceBase

	// The Site that owns this resources.  Resources belong to a site and cannot be shared across multiple sites
	Site *Site

	// The loader responsible for loading the doocument for this resource when needed
	Loader   ResourceLoader
	Renderer ResourceRenderer

	// Fullpath of the Resource uniquely identifying it within a Site
	FullPath string

	// Created timestamp on disk
	CreatedAt time.Time

	// Updated time stamp on disk
	UpdatedAt time.Time

	// When it was loaded (and parsed)
	LoadedAt time.Time

	// The ResourceState - Loaded, Pending, NotFound, Failed
	State int

	// Any errors with this resource (eg during load)
	Error error

	// os level Info about the resource
	info os.FileInfo

	// This will be set by the parser
	frontMatter FrontMatter
	Document    Document

	// The resource this is derived/copied/rendered from. This will only be set for output resources
	Source *Resource

	// True if the resource is parametric and can result in several instances
	IsParametric bool

	// If this is a parametric resources - this returns the space of all parameters
	// possible for this resource based on how it is loaded and its config it takes
	// For example a blog page of the form /a/b/c/[name].md could have 10 distinct values
	// for the "name" parameter.  Those will be populated here by the content processor
	// For now we are only looking at single level of parameters.  In the future we will
	// consider multiple parameters, eg: /[param1]/[param2]...
	ParamValues []string
	// Name of the parameter
	ParamName string

	NeedsIndex bool
	IsIndex    bool
}

// Load's the resource from disk including any front matter it might have.
/*
func (r *Resource) Load() *Resource {
	s := r.Site
	proc := s.GetResourceHandler(r)
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
*/

// Reset's the Resource's state to Pending so it can be reloaded
func (r *Resource) Reset() {
	r.State = ResourceStatePending
	r.info = nil
	r.Error = nil
	r.frontMatter.Loaded = false
	r.Document.Loaded = false
	r.Base = nil
	r.ParamValues = nil
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
	pos := r.frontMatter.Length

	fi, err := os.Open(r.FullPath)
	if err != nil {
		return nil, err
	}
	_, err = fi.Seek(pos, 0)
	return fi, err
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

// This methods add a new "parameter" to the resource.  Parametric resources are a way to ensure that a given reosurce (eg a page) can
// take several instances.  For example the content page `<content_root>/tags/[tag].md` can resultin multiple files of the form
// `<output_folder>/tags/tag1/index.html`, `<output_folder>/tags/tag2/index.html` and so on.  This is evaluated by rendering the
// the source file (`<content_root>/tags/[tag].md`)  in the "nameless" mode where the template would call the AddParam method
// once for each new child resources (eg tag1, tag2...)
func (r *Resource) AddParam(param string) *Resource {
	r.ParamValues = append(r.ParamValues, param)
	return r
}

// Very similar to the Addparam but allows adding a list of parameters in one call.
func (r *Resource) AddParams(params []string) *Resource {
	r.ParamValues = append(r.ParamValues, params...)
	return r
}

// Returns the path relative to the content root
func (r *Resource) RelPath() string {
	respath, found := strings.CutPrefix(r.FullPath, r.Site.ContentRoot)
	if !found {
		return ""
	}
	return respath
}

// Types of functions that filter resources (usually in a list call)
type ResourceFilterFunc func(res *Resource) bool

// Types of function used for sorting of resources.   returns true if a < b, false otherwise.
type ResourceSortFunc func(a *Resource, b *Resource) bool

// The default page type.  Each type can have its own page type and can be overridden in the Site.GetPage method.
type DefaultResourceBase struct {
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

func (page *DefaultResourceBase) LoadFrom(res *Resource) error {
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
	site := res.Site
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
