package core

import (
	"log"
	"os"
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
	Loaded            bool
	Data              map[string]any
	FrontMatterLength uint64
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

// Loads a resource and validates it.   Note that a resources may not
// necessarily be in memory just because it is loaded.  Just a Resource
// pointer is kept and it can be streamed etc
func (s *Site) GetResource(fullpath string) *Resource {
	res, found := s.resources[fullpath]
	if res == nil || !found {
		res = &Resource{
			FullPath:  fullpath,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			State:     ResourceStatePending,
		}
		s.resources[fullpath] = res
	}
	return res
}

func (r *Resource) Info() os.FileInfo {
	if r.info == nil {
		r.info, r.Error = os.Stat(r.FullPath)
		r.State = ResourceStateFailed
	}
	return r.info
}

func (r *Resource) FrontMatter() *FrontMatter {
	if !r.frontMatter.Loaded {
		f, err := os.Open(r.FullPath)
		if err != nil {
			r.Error = err
			r.State = ResourceStateFailed
		}
		rest, err := frontmatter.Parse(f, r.frontMatter.Data)
		log.Println("Rest: ", rest)
		if err != nil {
			r.Error = err
			r.State = ResourceStateFailed
		}
	}
	return &r.frontMatter
}
