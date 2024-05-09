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
	gfn "github.com/panyam/goutils/fn"
	"github.com/radovskyb/watcher"
)

var ErrContentPathInvalid = fmt.Errorf("Invalid content path")

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

	NoLiveReload bool

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

	// All files including published files will be served from here!
	filesRouter *mux.Router

	// This router wraps "published" files to ensure that when source
	// files have changed - it recompiles them before calling the underlying
	// file handler - only for non-static files
	reloadWatcher *watcher.Watcher
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
		x := s.filesRouter.PathPrefix("/")
		x.Handler(http.StripPrefix("/", fileServer))
	}
	return s.filesRouter
}

// Instead of
func (s *Site) StopWatching() {
	if s.reloadWatcher == nil {
		s.reloadWatcher.Close()
		s.reloadWatcher = nil
	}
}

func (s *Site) StartWatching() {
	if s.reloadWatcher == nil {
		w := watcher.New()
		s.reloadWatcher = w

		// SetMaxEvents to 1 to allow at most 1 event's to be received
		// on the Event channel per watching cycle.
		//
		// If SetMaxEvents is not set, the default is to send all events.
		w.SetMaxEvents(1)

		// Only notify rename and move events.
		// w.FilterOps(watcher.Rename, watcher.Move)

		// Only files that match the regular expression during file listings
		// will be watched.
		// r := regexp.MustCompile("^abc$")
		// w.AddFilterHook(watcher.RegexFilterHook(r, false))

		// Start that handling process
		go func() {
			for {
				select {
				case event := <-w.Event:
					fmt.Println(event) // Print the event's info.
				case err := <-w.Error:
					log.Fatalln(err)
				case <-w.Closed:
					return
				}
			}
		}()

		// start the watching process
		if err := w.Start(time.Millisecond * 100); err != nil {
			log.Fatalln(err)
		}

		if err := w.AddRecursive(s.ContentRoot); err != nil {
			log.Fatalln(err)
		}
	}

	/*
		parts := strings.Split(r.URL.Path, "/")[1:]
		log.Println("222 - Default Handler URL Parts: ", parts)

		// Static folder so nothing to do
		for i := 0; i < len(s.StaticFolders); i += 2 {
			if strings.HasPrefix(r.URL.Path, s.StaticFolders[i]) {
				log.Println("Serving static file, no rebuild needed: ", r.URL.Path)
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		realPath, isDir, err := s.PathToDir(r.URL.Path)
		log.Println("Real File Path, IsDir, Err: ", realPath, isDir, err)
	*/

	// We can go two ways:
	// 1. realPath must be either in one of the static folders or
	// 2. P1 := in OutputDir/<r.URL.Path>
	//
	// If this path P1 does not exist, we dont know if it is a file or a dir
	// if it exists then serve it
	// if it is a dir and dir/index.html exists serve it - is "index.html" hard coded?
	// if it does not exist, try to rebuild it.
	// Challenge is how do we know for a given entry in the output dir
	// a: whether it *should* be file or a directory with files in it - ie is it a page?
	// b: Given a directory, how do we know which sources files it maps to?
	//
	// May be we can do a few checks:
	// 1. we can assume it is always in ContentRoot and not in StaticFolders,
	// because if it is in StaticFolders we can just go ahead
	// 2. Let say we have something like this: <OutputDir>/<PathToFile>
	// if <OutputDir><PathToFile> AND <ContentRoot>/<PathToFile> are files then copy_if_older and serve
	// if <OutputDir><PathToFile> is a dir - it means its counter part <CR>/<P2F> is either a dir
	// or a file
	// if it is a file it means <CR>/<P2F> is a "leaf" page which needs to be saved into a dir with
	// an index.html.
	// if source leaf is newer - then regenerate
	// server dest leaf/index.html
	// if both are dirs - then check respective index.xyz files and regenerate if necessary before
	// serving
	//
	// Also may be useful to see if we need a "sitemap" data structure to capture dependencies

	/*
		info, err := os.Stat(realPath)
		if err != nil {
			s.ServeError(w, r, err)
			return
		}

		if !strings.HasPrefix(info.Name(), s.ContentRoot) {
			// How should we handle this error?
			// this is outside the content directory
			// TODO - may be allow a list of "kosher" dirs outside content root?
			// or make ContentRoot a list?
			http.Error(w, "File is a symlink outside ContentRoot", http.StatusBadRequest)
			return
		}

		if info.IsDir() {
			s.ServeDir(w, r, realPath, info)
		} else {
			s.ServeFile(w, r, realPath, info)
		}
	*/
}

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

func (s *Site) Load() {
	var foundResources []*Resource
	inputBundle := (&ResourceBundle{RootDir: s.ContentRoot, Name: "Sources"}).Init()
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
		res, err2 := inputBundle.LoadResource(fullpath)
		log.Println("ResLoad, Err: ", res, err2)

		// TODO - refer to cache if this need to be rebuilt? or let Rebuild do it?
		foundResources = append(foundResources, res)

		return nil
	})

	s.Rebuild(foundResources)
}

func (s *Site) GetContentProcessor(rs *Resource) ContentProcessor {

	// normal file
	// check type and call appropriate processor
	// Should we call processor directly here or collect a list and
	// pass that to Rebuild with those resources?
	ext := filepath.Ext(rs.Path)
	log.Println("Name, Ext: ", rs.Path, ext)
	return nil
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
		for _, r := range rs {
			handler := s.GetContentProcessor(r)
			if handler == nil {
				log.Println("Processor not found for resource: ", r.Path)
				continue
			}
			nextResources, err := handler.Process(r, s)
			if err != nil {
				errors = append(errors, err)
			} else {
				for _, newr := range nextResources {
					nextGenResources[newr] = true
				}
			}
		}
		rs = gfn.MapKeys(nextGenResources)
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
