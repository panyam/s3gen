package core

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/adrg/frontmatter"
)

const (
	ResourceStatePending = iota
	ResourceStateLoaded
	ResourceStateDeleted
	ResourceStateNotFound
	ResourceStateFailed
)

type FrontMatter struct {
	Loaded bool
	Data   map[string]any
	Length int64
}

type ContentProcessor interface {
	// Called to handle a resource -
	// This can generate more resources
	IsIndex(s *Site, res *Resource) bool
	NeedsIndex(s *Site, res *Resource) bool
	LoadPage(res *Resource, page *Page) error
	// Process(s *Site, inres *Resource, writer io.Writer) error
}

// Our interface for returning all static content in our site
type ResourceService interface {
	GetResource(fullpath string) *Resource
	ListResources(filterFunc func(res *Resource) bool,
		sortFunc func(a *Resource, b *Resource) bool,
		offset int, count int) []*Resource
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

	// The resource that corresponds to this page
	// TODO - Should this be just the root resource or all resources for it?
	Content *Resource

	// Tells whether this is a detail page or a listing page
	IsListPage bool

	// The root view that corresponds to this page
	// By default - we use the BasePage view
	RootView View

	// Next page after this for navigation
	PrevPage *Page

	// Previous page for navigation
	NextPage *Page
}

// A ResourceBundle is a collection of resources all nested under a single
// root directory.
type ResourceBundle struct {
	// Name of this resource bundle
	Name string

	// Root directory where the resources are nested under.
	// Not *every* file is implicitly loaded.  To add a resource
	// in a bundle it has to be Loaded first.
	RootDir string
}

/**
 * Each resource in our static site is identified by a unique path.
 * Note that resources can go through multiple transformations
 * resulting in more resources - to be converted into other resources.
 * Each resource is uniquely identified by the Bundle + Path combination
 */
type Resource struct {
	FullPath    string // Unique URI/Path
	BundleName  string
	ContentType string

	// Created timestamp on disk
	CreatedAt time.Time

	// Updated time stamp on disk
	UpdatedAt time.Time

	// will be set to when it was last processed if no errors occurred
	ProcessedAt time.Time

	// Loaded, Pending, NotFound, Failed
	State int

	// Resources this one depends on - to determine if a rebuild is needed
	// If a resource does not depend on any others then this is a root
	// resource
	DependsOn map[string]bool

	// Any errors with this resource
	Error error

	// Info about the resource
	info os.FileInfo

	// Marks whether front matter was loaded
	frontMatter FrontMatter
}

func (r *Resource) Reset() {
	r.State = ResourceStatePending
	r.info = nil
	r.Error = nil
	r.frontMatter.Loaded = false
}

// Ensures that a resource's parent directory exists
func (r *Resource) EnsureDir() {
	dirname := filepath.Dir(r.FullPath)
	if err := os.MkdirAll(dirname, 0755); err != nil {
		log.Println("Error creating dir: ", dirname, err)
	}
}

func (r *Resource) DirName() string {
	return filepath.Dir(r.FullPath)
}

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
		r.State = ResourceStateFailed
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
	return false
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
		}
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
	return &r.frontMatter
}
