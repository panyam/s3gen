package s3gen

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/adrg/frontmatter"
	gfn "github.com/panyam/goutils/fn"
	"gopkg.in/yaml.v2"
)

// ResourceLoader is an interface for loading and parsing a resource.
type ResourceLoader interface {
	Load(res *Resource) error
}

// ResourceProcessor is an interface for generating target resources from a source resource.
type ResourceProcessor interface {
	GenerateTargets(res *Resource, deps map[string]*Resource) error
}

// ResourceRenderer is an interface for rendering a resource to an output stream.
type ResourceRenderer interface {
	Render(res *Resource, w io.Writer) error
}

// ResourceBase is an interface for the base data structure of a resource.
type ResourceBase interface {
	LoadFrom(*Resource) error
}

// FrontMatter represents the parsed front matter of a resource.
type FrontMatter struct {
	// Loaded is true if the front matter has been loaded and parsed.
	Loaded bool

	// Data is a map containing the parsed front matter data.
	Data map[string]any

	// Length is the length of the front matter in bytes.
	Length int64
}

// Document represents the parsed content of a resource (e.g., an AST).
type Document struct {
	// Loaded is true if the document has been loaded and parsed.
	Loaded bool

	// LoadedAt is the time the document was loaded.
	LoadedAt time.Time

	// Root is the root of the parsed document tree.
	Root any

	// Metadata is a map for storing extra metadata about the document,
	// such as a table of contents.
	Metadata map[string]any
}

// SetMetadata sets a metadata key-value pair on the document.
func (d *Document) SetMetadata(k string, v any) {
	if d.Metadata == nil {
		d.Metadata = map[string]any{}
	}
	d.Metadata[k] = v
}

const (
	// ResourceStatePending is the initial state of a resource before it is loaded.
	ResourceStatePending = iota

	// ResourceStateLoaded is the state of a resource that has been successfully loaded.
	ResourceStateLoaded

	// ResourceStateDeleted is the state of a resource that has been deleted.
	ResourceStateDeleted

	// ResourceStateNotFound is the state of a resource that could not be found.
	ResourceStateNotFound

	// ResourceStateFailed is the state of a resource that failed to load.
	ResourceStateFailed
)

// Resource represents a single file in the site, such as a content file,
// a template, or a static asset.
type Resource struct {
	// Base is the underlying data structure for the resource.
	Base ResourceBase

	// Site is the site that this resource belongs to.
	Site *Site

	// Loader is the loader responsible for loading the resource's document.
	Loader ResourceLoader
	// Renderer is the renderer responsible for rendering the resource.
	Renderer ResourceRenderer

	// FullPath is the absolute path to the resource file.
	FullPath string

	// CreatedAt is the timestamp when the resource was created on disk.
	CreatedAt time.Time

	// UpdatedAt is the timestamp when the resource was last updated on disk.
	UpdatedAt time.Time

	// LoadedAt is the timestamp when the resource was loaded and parsed.
	LoadedAt time.Time

	// State is the current state of the resource in the build process.
	State int

	// Error holds any error that occurred while processing the resource.
	Error error

	// info is the os.FileInfo for the resource.
	info os.FileInfo

	// frontMatter holds the parsed front matter of the resource.
	frontMatter FrontMatter
	// Metadata is a map for storing arbitrary metadata about the resource.
	Metadata map[string]any
	// Document is the parsed document of the resource.
	Document Document

	// Source is the resource that this resource was derived from. This is
	// only set for output resources.
	Source *Resource

	// IsParametric is true if the resource is a parametric page.
	IsParametric bool

	// ParamValues is the list of parameter values for a parametric page.
	ParamValues []string
	// ParamName is the name of the parameter for a parametric page.
	ParamName string

	// NeedsIndex is true if the resource should be rendered as an index page.
	NeedsIndex bool
	// IsIndex is true if the resource is an index page.
	IsIndex bool
}

// Reset resets the resource's state to Pending so it can be reloaded.
func (r *Resource) Reset() {
	r.State = ResourceStatePending
	r.info = nil
	r.Error = nil
	r.Metadata = map[string]any{}
	r.frontMatter.Loaded = false
	r.Document.Loaded = false
	r.Base = nil
	r.ParamValues = nil
}

// SetMetadata sets a metadata key-value pair on the resource.
func (r *Resource) SetMetadata(key string, value any, kvpairs ...any) any {
	// log.Printf("Settin Key %s in resource %s", key, res.FullPath)
	if r.Metadata == nil {
		r.Metadata = map[string]any{}
	}
	r.Metadata[key] = value
	for i := 0; i < len(kvpairs); i += 2 {
		key = kvpairs[i].(string)
		value = kvpairs[i+1]
		r.Metadata[key] = value
		// log.Printf("Settin Key %s in resource %s", key, res.FullPath)
	}
	return ""
}

// EnsureDir ensures that the resource's parent directory exists.
func (r *Resource) EnsureDir() {
	dirname := filepath.Dir(r.FullPath)
	if err := os.MkdirAll(dirname, 0755); err != nil {
		log.Println("Error creating dir: ", dirname, err)
	}
}

// DirName returns the resource's directory.
func (r *Resource) DirName() string {
	return filepath.Dir(r.FullPath)
}

// WithoutExt returns the resource's path without the extension.
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

// Info returns the os.FileInfo for the resource.
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

// ReadAll reads all the content of the resource after the front matter.
func (r *Resource) ReadAll() ([]byte, error) {
	reader, err := r.Reader()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

// Reader returns a reader for the content of the resource after the front matter.
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

// IsDir returns true if the resource is a directory.
func (r *Resource) IsDir() bool {
	return r.Info().IsDir()
}

// Ext returns the extension of the resource's path.
func (r *Resource) Ext() string {
	return filepath.Ext(r.FullPath)
}

// FrontMatter returns the parsed front matter of the resource, loading it if necessary.
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
			// rest, err := frontmatter.Parse(f, r.frontMatter.Data)
			rest, err := frontmatter.Parse(f, r.frontMatter.Data, DefaultFormats...)
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

// AddParam adds a new parameter value to a parametric resource. This is used
// during the parameter discovery phase of rendering a parametric page.
func (r *Resource) AddParam(param string) *Resource {
	r.ParamValues = append(r.ParamValues, param)
	return r
}

// AddParams adds a list of parameter values to a parametric resource.
func (r *Resource) AddParams(params []string) *Resource {
	r.ParamValues = append(r.ParamValues, params...)
	return r
}

// RelPath returns the path of the resource relative to the content root.
func (r *Resource) RelPath() string {
	respath, found := strings.CutPrefix(r.FullPath, r.Site.ContentRoot)
	if !found {
		return ""
	}
	return respath
}

// ResourceFilterFunc is a function type for filtering resources.
type ResourceFilterFunc func(res *Resource) bool

// ResourceSortFunc is a function type for sorting resources.
type ResourceSortFunc func(a *Resource, b *Resource) bool

// DefaultResourceBase is the default implementation of the ResourceBase interface.
type DefaultResourceBase struct {
	// Slug is the URL-friendly slug for the page.
	Slug string

	// Title is the title of the page.
	Title string

	// Link is the permalink to the page.
	Link string

	// Summary is a short summary of the page.
	Summary string

	// CreatedAt is the creation timestamp of the page.
	CreatedAt time.Time
	// UpdatedAt is the last modification timestamp of the page.
	UpdatedAt time.Time

	// IsDraft is true if the page is a draft.
	IsDraft bool

	// CanonicalUrl is the canonical URL for the page.
	CanonicalUrl string

	// Tags is a list of tags for the page.
	Tags []string

	// Res is the resource that this base is associated with.
	Res *Resource

	// State is the current state of the resource.
	State int

	// Error holds any error that occurred while processing the resource.
	Error error
}

// LoadFrom loads the data for the DefaultResourceBase from a resource's front matter.
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

var DefaultFormats []*frontmatter.Format = []*frontmatter.Format{
	// YAML.
	&frontmatter.Format{"---", "---", yaml.Unmarshal, false, false},
	&frontmatter.Format{"---yaml", "---", yaml.Unmarshal, false, false},
	// TOML.
	&frontmatter.Format{"+++", "+++", toml.Unmarshal, false, false},
	&frontmatter.Format{"---toml", "---", toml.Unmarshal, false, false},
	// JSON.
	&frontmatter.Format{";;;", ";;;", json.Unmarshal, false, false},
	&frontmatter.Format{"---json", "---", json.Unmarshal, false, false},
}
