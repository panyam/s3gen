package core

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	gut "github.com/panyam/goutils/utils"
)

// Our static site type that holds metadata about all input directories
// output directories, implicit/explict routes, templates, etc
// Unlike a dynamic site served by go routers etc, the point of this
// is to use the same routers but to generate all this as static content
// so we can serve them directly without needing a router - faster/cdn
// and other benefits

// Usage:
//
//	router = mux.NewRouter()
//	s := Site{.....}
//	s.SetupRoutes(router)
//	srv := http.Server{
//		Processor: router,
//	 Addr: ":8080",
//	}
//	srv.ListenAndServe()
type Site struct {
	// What are the elements of a static site?

	// ContentRoot is the root of all your pages.
	// One structure we want to place is use folders to emphasis url structure too
	// so a site with mysite.com/a/b/c/d
	// would have the file structure:
	// <ContentRoot>/a/b/c/d.<content_type>
	ContentRoot string

	// Final output directory where resources are generated/published to
	OutputDir string

	// A directory used by the sitegen to hold cached info etc
	// TODO - easy to do just do sqlite?
	CacheDir string

	// The http path prefix the site is prefixed in,
	// eg
	//		myblog.com								=> PathPrefix = "/"
	//		myblog.com/blog						=> PathPrefix = "/blog"
	//		myblog.com/blogs/blog1		=> PathPrefix = "/blogs/blog1"
	//
	// There is no restriction on this.  There could be other routes (eg /products page)
	// that could be served by a different router all together in parallel to say /blog.
	// This is only needed so that the generator knows where to "root" the blog in the URL
	PathPrefix string

	// ContentProcessors tell us how to "process" a content of a given type.
	// types are denoted by extensions for now later on we could do something else
	ContentProcessors map[string]ContentProcessor
}

// Sets up the routes required for this site "under" a given router
func (s *Site) SetupRoutes(router *mux.Router) {
	// we are directly mapping PathPrefix -> OutputDir
	// TODO - may be do individual routes for sub sections etc?
	// How does it handle all the different index.htmls?
	// TIL - this also serves either a directory listing if x/y/d and d is a dir
	// or index.html if d/index.html exists.  Nice
	router.PathPrefix(s.PathPrefix).
		Handler(http.StripPrefix(
			s.PathPrefix,
			http.FileServer(http.Dir(s.OutputDir)),
		))
}

func (s *Site) Load() {
	// keep a map of files encountered and their statuses
	filepath.WalkDir("./", func(root string, info os.DirEntry, err error) error {
		if err != nil {
			// just print err related to the path and stop scanning
			// if this err means something else we can do other things here
			log.Println("Error in path: ", info, err)
			return err
		}

		if info.IsDir() {
			// see if we should this directory
			// eg if this is a special dir we may give it a different treatment
			// and return a SkipDir
			return nil
		}

		// normal file
		// check type and call appropriate processor
		// Should we call processor directly here or collect a list and
		// pass that to Rebuild with those resources?
		ext := filepath.Ext(info.Name())
		log.Println("Name, Ext: ", info.Name(), ext)

		return nil
	})
}

// When a resource is updated, it has to do a couple of things
// First get a transitive closure of all resource that depend on it
// then call the "build" on that resource - which in turn would load
// all of its dependencies
// So this is a closure operation on sets of resources each time
func (s *Site) Rebuild(rs []*Resource) {
	var errors []error
	for len(rs) > 0 {
		nextGenResources := make(map[*Resource]bool)
		for r := range rs {
			handler := s.GetContentProcessor(r)
			nextResources, err := handler.Process(r)
			if err != nil {
				errors = append(errors, err)
			} else {
				for newr := range nextResources {
					nextGenResources[newr] = true
				}
			}
		}
		rs = gut.MapValues(nextGenResources)
	}
}

// So we know how to "start" it.  What should it actually do:
// 1. Go through all files in ContentRoot and collect files first
// 2. Detect file type either based on extension or load it and check
// 3. For each loaded file "process" and "spit" it and update our cache db
// 4.
// Code gen and publish a site
func (s *Site) Publish() {
}
