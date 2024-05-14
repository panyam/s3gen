package core

import (
	"fmt"
	"log"
	"os"
	"time"

	gfn "github.com/panyam/goutils/fn"
	"github.com/radovskyb/watcher"
)

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

		go func() {
			buildFreq := s.BuildFrequency
			if buildFreq <= 0 {
				buildFreq = 1000 * time.Millisecond
			}
			tickerChan := time.NewTicker(buildFreq)
			defer tickerChan.Stop()

			foundResources := make(map[string]*Resource)
			for {
				select {
				case event := <-w.Event:
					fmt.Println(event) // Print the event's info.
					log.Println("Collecting Event: ", event)

					fullpath := event.Path
					info, err := os.Stat(fullpath)
					if err != nil {
						fmt.Println(err)
						return
					}

					// only deal with files
					if !info.IsDir() && (s.IgnoreFileFunc == nil || !s.IgnoreFileFunc(fullpath)) {
						res := s.GetResource(fullpath)
						if res != nil {
							// map fullpath to a resource here

							// TODO - refer to cache if this need to be rebuilt? or let Rebuild do it?
							foundResources[fullpath] = res
						}
					}
				case err := <-w.Error:
					log.Fatalln(err)
				case <-w.Closed:
					// Stop building and uit
					return
				case <-tickerChan.C:
					// if we have things in the collected files - kick off a rebuild
					if len(foundResources) > 0 {
						log.Println("files collected so far: ", foundResources)

						s.Rebuild(gfn.MapValues(foundResources))
						// reset changed files
						foundResources = make(map[string]*Resource)
					}
					break
				}
			}
		}()

		log.Println("Adding files recursive: ", s.ContentRoot)
		if err := w.AddRecursive(s.ContentRoot); err != nil {
			log.Fatalln("Error adding files recursive: ", s.ContentRoot, err)
		}

		// start the watching process
		go func() {
			log.Println("Starting watcher...")
			if err := w.Start(time.Millisecond * 100); err != nil {
				log.Fatal("Error starting watcher...", err)
			}
		}()
	}
}
