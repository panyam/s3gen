package core

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
	gut "github.com/panyam/goutils/utils"
	"github.com/radovskyb/watcher"
)

var ErrContentPathInvalid = fmt.Errorf("Invalid content path")

type ContentProcessor interface {
	// Called to handle a resource -
	// This can generate more resources
	IsIndex(s *Site, res *Resource) bool
	NeedsIndex(s *Site, res *Resource) bool
	Process(s *Site, inres *Resource, outres *Resource) error
}

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
//	s.SetupRoutes.filesRouter)
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

	// Defines where all static assets are to be used from
	AssetsRoot string

	// Final output directory where resources are generated/published to
	OutputDir string

	// A directory used by the sitegen to hold cached info etc
	// TODO - easy to do just do sqlite?
	CacheDir string

	LiveReload bool
	LazyLoad   bool

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

	IgnoreDirFunc  func(dirpath string) bool
	IgnoreFileFunc func(filepath string) bool

	StaticFolders []string

	BuildFrequency time.Duration

	// All files including published files will be served from here!
	filesRouter *mux.Router

	// This router wraps "published" files to ensure that when source
	// files have changed - it recompiles them before calling the underlying
	// file handler - only for non-static files
	reloadWatcher *watcher.Watcher

	resources map[string]*Resource
}

func (s *Site) Init() *Site {
	s.ContentRoot = gut.ExpandUserPath(s.ContentRoot)
	s.OutputDir = gut.ExpandUserPath(s.OutputDir)
	if s.resources == nil {
		s.resources = make(map[string]*Resource)
	}
	return s
}

// https://benhoyt.com/writings/go-routing/#split-switch
func (s *Site) GetRouter() *mux.Router {
	if s.filesRouter == nil {
		s.filesRouter = mux.NewRouter()

		// Setup local/static paths
		for i := 0; i < len(s.StaticFolders); i += 2 {
			path, folder := s.StaticFolders[i], s.StaticFolders[i+1]
			log.Printf("Adding static route: %s -> %s", path, folder)
			s.filesRouter.PathPrefix(path).Handler(
				http.StripPrefix(path, http.FileServer(http.Dir(folder))))
		}

		// Serve everything else from the

		// Now add the file loader/handler for the "published" dir
		fileServer := http.FileServer(http.Dir(s.OutputDir))
		realHandler := http.StripPrefix("/", fileServer)
		if s.LazyLoad {
			/*
				x := s.filesRouter
				x.PathPrefix("").Subrouter().Use(func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						log.Println("Doing some decorator: ", r.URL.Path)
						next.ServeHTTP(w, r)
					})
				})
				// s.filesRouter.PathPrefix("/")
				x.Handle("/", realHandler)
			*/
			x := s.filesRouter.PathPrefix("/")
			x.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				log.Println("Ensuring path is built: ", r.URL.Path, "Site Prefix: ", s.PathPrefix)

				srcRes := s.urlPathToFilePath(r.URL.Path)
				log.Println("Source Resource: ", srcRes)
				// What should happen here?
				realHandler.ServeHTTP(w, r)
			}))
		} else {
			x := s.filesRouter.PathPrefix("/")
			x.Handler(realHandler)
		}
	}
	return s.filesRouter
}

// Given a file in our file system - returns the "URL" that can serve that file.
func (s *Site) filePathToUrlPath(filePath string) string {
	res := s.GetResource(filePath)
	log.Println("Res, Info: ", res, res.Info())
	return ""
}

// Given a url path, eg "/a/b/c/d/e" return which "root" resource it can be
// served by.  The "root" resource is the file that will be used to compile
// and serve this file.   Eg if we have (assuming our contentRoot is ./data and
// site root is /blog
//
//	/blog/b/c/d/
//
// Then we expect the path to be at: ./data/b/c/d/<index.ext>
//
// <index.ext> could be one of the "index" files we designate,
// eg _index.html or index.html or _index.md or index.md etc
//
// For now we assume only one of these files exist and it is upto
// the page oragnizer to pick this.  An exception for this is if we want things like
func (s *Site) urlPathToFilePath(urlPath string) *Resource {
	cand := filepath.Join(s.ContentRoot, urlPath)
	log.Println("Cand: ", cand)
	// if cand is directory - see if cand/<index_html> exists
	return nil
}

// The base entry point for a serving a site with our customer handler
// This is used for a few  purposes:
//  1. If you want to serve a static site but want to to have a "refresh" handler.  ie if a source changes, this can catch it and rebuild the necessary things before serving it
//     2.
func (s *Site) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// The entry point router for our site
	parts := strings.Split(r.URL.Path, "/")[1:]
	log.Println("1112 - URL Parts: ", parts)
	s.GetRouter().ServeHTTP(w, r)
}

// Walks through all our resources and rebuilds/republishes the static site
func (s *Site) Load() *Site {
	var foundResources []*Resource
	// keep a map of files encountered and their statuses
	filepath.WalkDir(s.ContentRoot, func(fullpath string, info os.DirEntry, err error) error {
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
			if s.IgnoreDirFunc != nil && s.IgnoreDirFunc(fullpath) {
				return filepath.SkipDir
			}
			return nil
		}

		if s.IgnoreFileFunc != nil && s.IgnoreFileFunc(fullpath) {
			return nil
		}

		// map fullpath to a resource here
		res := s.GetResource(fullpath)

		// TODO - refer to cache if this need to be rebuilt? or let Rebuild do it?
		foundResources = append(foundResources, res)

		return nil
	})

	s.Rebuild(foundResources)
	return s
}

// This is the heart of the build process.   This method is called with a list of resources that
// have to be reprocessed (either due to periodic updates or change events etc).   Resources in
// our site form a graph and each resource is processed by a ContentProcessor appropriate for it
// The content processor can create more resources that may need an update because they are
// dependant on this resource.   By allowing a list of resources to be processed in a batch
// it is more efficient to perform batch dependencies instead of doing repeated builds for each
// change.
// First get a transitive closure of all resource that depend on it
// then call the "build" on that resource - which in turn would load
// all of its dependencies
// So this is a closure operation on sets of resources each time
func (s *Site) Rebuild(rs []*Resource) {
	// var errors []error

	for _, res := range rs {
		proc := s.GetContentProcessor(res)
		if proc == nil {
			log.Println("Processor not found for resource: ", res.FullPath)
			continue
		}
		outres := s.DestResourceFor(res)
		if outres != nil {
			// this is an index page
			if err := proc.Process(s, res, outres); err != nil {
				log.Printf("error processing %s: %v", res.FullPath, err)
			}
		}
	}
	/*
		for len(rs) > 0 {
			nextGenResources := make(map[*Resource]bool)
			for _, r := range rs {
				handler := s.GetContentProcessor(r)
				if handler == nil {
					log.Println("Processor not found for resource: ", r.FullPath)
					continue
				}
				nextResources, err := handler.Process(s, r)
				if err != nil {
					errors = append(errors, err)
				} else {
					for _, newr := range nextResources {
						nextGenResources[newr] = true
						// also add it to our map
						s.resources[newr.FullPath] = newr
					}
				}
			}
			rs = gfn.MapKeys(nextGenResources)
		}
	*/
}

func (s *Site) DestResourceFor(res *Resource) *Resource {
	// if a resource is in the content root - then return its "equiv" in the output
	// This also ensures that we have problem "Foo/index.html" for Foo.md files
	respath, found := strings.CutPrefix(res.FullPath, s.ContentRoot)
	if !found {
		log.Println("H1: ", res.FullPath, s.ContentRoot)
		return nil
	}

	if res.Info() == nil {
		log.Println("here 2....")
		return nil
	}

	proc := s.GetContentProcessor(res)
	destpath := ""
	if res.Info().IsDir() {
		// Then this will be served with dest/index.html
		destpath = filepath.Join(s.OutputDir, respath)
	} else if proc.IsIndex(s, res) {
		destpath = filepath.Join(s.OutputDir, filepath.Dir(respath), "index.html")
	} else if proc.NeedsIndex(s, res) {
		// res is not a dir - eg it something like xyz.ext
		// depending on ext - if the ext is for a page file
		// then generate OutDir/xyz/index.html
		// otherwise OutDir/xyz.ext
		ext := filepath.Ext(respath)

		rem := respath[:len(respath)-len(ext)]

		// TODO - also see if there is a .<lang> prefix on rem after ext has been removed
		// can use that for language sites
		destpath = filepath.Join(s.OutputDir, rem, "index.html")
		log.Println("ResP: ", respath, "Remaining: ", rem)
	} else {
		// basic static file - so copy as is
		destpath = filepath.Join(s.OutputDir, respath)
	}
	// log.Println("Res, Dest Paths: ", respath, destpath)
	return s.GetResource(destpath)
}

func (s *Site) GetContentProcessor(rs *Resource) ContentProcessor {
	// normal file
	// check type and call appropriate processor
	// Should we call processor directly here or collect a list and
	// pass that to Rebuild with those resources?
	ext := filepath.Ext(rs.FullPath)

	// TODO - move to a table lookup or regex based one
	if ext == ".mdx" || ext == ".md" {
		return NewMDContentProcessor("")
	}
	if ext == ".html" || ext == ".htm" {
		return &HTMLContentProcessor{}
	}
	log.Println("Could not find proc for, Name, Ext: ", rs.FullPath, ext)
	return nil
}

// /////////////////// ATTIC

func (s *Site) PathToDir(path string) (string, bool, error) {
	startingDir := filepath.SplitList(s.ContentRoot)
	minPartsNeeded := len(startingDir)
	for _, p := range strings.Split(path, "/") {
		if p == "." {
			// do nothing
		} else if p == ".." {
			if len(startingDir) == minPartsNeeded {
				// cannot go any further so return error
				return "", false, ErrContentPathInvalid
			} else {
				startingDir = startingDir[:len(startingDir)-1]
			}
		} else {
			// append to it
			startingDir = append(startingDir, p)
		}
	}
	return filepath.Join(startingDir...), true, nil
}

func (s *Site) ServeFile(w http.ResponseWriter, r *http.Request, realPath string, file fs.FileInfo) {
	// TODO
	// 1. get type
	// 2. get content processor
	// 3. Kick off compilation and serve
}

func (s *Site) ServeDir(w http.ResponseWriter, r *http.Request, realPath string, file fs.FileInfo) {
	// TODO
	// 1. look for index html/md/mdx/htm/etc - if it exists
	// 2. if it - then ServeFile it with above func
	// 3. else - see if there is a "handler" in code for it and call it
	// 4. otherwise see if we have a default handler for "listing" types - then call it
	// 5. else 404
	// for _, idx := range s.IndexFileNames { }
}

func (s *Site) ServeError(w http.ResponseWriter, r *http.Request, err error) {
	if os.IsNotExist(err) {
		http.Error(w, "Not Found", http.StatusNotFound)
	} else {
		log.Println("Internal error: ", err)
		http.Error(w, "Internal Error", http.StatusInternalServerError)
	}
}
