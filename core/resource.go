package core

import (
	"fmt"
	"time"
)

type ContentProcessor interface {
	// Called to handle a resource -
	// This can generate more resources
	Process(res *Resource, s *Site) ([]*Resource, error)
}

const (
	ResourceStatePending = iota
	ResourceStateLoaded
	ResourceStateNotFound
	ResourceStateFailed
)

/**
 * Each resource in our static site is identified by a unique path.
 * Note that resources can go through multiple transformations
 * resulting in more resources - to be converted into other resources.
 * Each resource is uniquely identified by the Bundle + Path combination
 */
type Resource struct {
	Bundle      *ResourceBundle
	Path        string
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
}

func (r *Resource) Id() string {
	return fmt.Sprintf("%s:%s", r.Bundle.Name, r.Path)
}

// A ResourceBundle is a collection of resources all nested under a single
// root directory.
type ResourceBundle struct {
	// Root directory where the resources are nested under.
	// Not *every* file is implicitly loaded.  To add a resource
	// in a bundle it has to be Loaded first.
	RootDir string

	// Name of this resource bundle
	Name string

	// collection of all loaded/"tried" resources in our bundle
	resources map[string]*Resource
}

func (rb *ResourceBundle) Init() *ResourceBundle {
	if rb.resources == nil {
		rb.resources = make(map[string]*Resource)
	}
	return rb
}

// Loads a resource and validates it.   Note that a resources may not
// necessarily be in memory just because it is loaded.  Just a Resource
// pointer is kept and it can be streamed etc
func (rb *ResourceBundle) LoadResource(relpath string) (*Resource, error) {
	res, found := rb.resources[relpath]
	if res == nil || !found {
		res = &Resource{
			Bundle:    rb,
			Path:      relpath,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			State:     ResourceStatePending,
		}
		rb.resources[relpath] = res
	}
	return res, nil
}
