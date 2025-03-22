package s3gen

import (
	"time"
)

// Loads a resource and validates it.   Note that a resources may not
// necessarily be in memory just because it is loaded.  Just a Resource
// pointer is kept and it can be streamed etc
func (s *Site) GetResource(fullpath string) *Resource {
	res, found := s.resources[fullpath]
	if res == nil || !found {
		res = &Resource{
			Site:      s,
			FullPath:  fullpath,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			State:     ResourceStatePending,
		}
		s.resources[fullpath] = res
	}

	return res
}

// Remove a resources from this graph along with all its dependencies
func (s *Site) RemoveResource(path string) *Resource {
	r := s.resources[path]
	s.RemoveEdgesTo(path)
	s.RemoveEdgesFrom(path)
	return r
}

func (s *Site) PathExists(srcpath string, destpath string) bool {
	if s.resedges == nil {
		return false
	}
	q := []string{srcpath}
	for len(q) > 0 {
		var nq []string
		for _, p := range q {
			edges := s.resedges[p]
			for _, next := range edges {
				if next == destpath {
					return true
				} else {
					nq = append(nq, next)
				}
			}
		}
		q = nq
	}
	return false
}

// Add a dependency edge between two resources identified by their full paths.
// Returns true if edge was added without incurring a cycle,
// returns false if edge would have resulted in a cycle.
func (s *Site) AddEdge(srcpath string, destpath string) bool {
	if s.PathExists(destpath, srcpath) {
		return false
	}
	if s.EdgeExists(srcpath, destpath) {
		return true
	}
	s.resedges[srcpath] = append(s.resedges[srcpath], destpath)
	return true
}

// Returns true if an edge exists between a source and a destination resource
func (s *Site) EdgeExists(srcpath string, destpath string) bool {
	if s.resedges == nil {
		s.resedges = make(map[string][]string)
	}
	if s.resedges[srcpath] != nil {
		for _, n := range s.resedges[srcpath] {
			if n == destpath {
				// already exists
				return true
			}
		}
	}
	return false
}

// Removes a dependency edge between two resources identified by their full paths
func (s *Site) RemoveEdge(srcpath string, destpath string) bool {
	if s.resedges == nil || s.resedges[srcpath] == nil {
		return false
	}
	for index, n := range s.resedges[srcpath] {
		if n == destpath {
			// already exists
			slice := s.resedges[srcpath]
			s.resedges[srcpath] = append(slice[:index], slice[index+1:]...)
			return true
		}
	}
	return false
}

// Removes all resources that a given path depends on
func (s *Site) RemoveEdgesTo(destpath string) {
	for srcpath := range s.resedges {
		s.RemoveEdge(srcpath, destpath)
	}
}

// Remove all resources depended by a given path
func (s *Site) RemoveEdgesFrom(srcpath string) {
	if s.resedges != nil {
		s.resedges[srcpath] = nil
	}
}
