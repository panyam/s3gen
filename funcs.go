package s3gen

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"path/filepath"
	"sort"
	"strings"

	gfn "github.com/panyam/goutils/fn"
	gut "github.com/panyam/goutils/utils"
	gotl "github.com/panyam/templar"
)

// A few template functions attached to the site itself

func (s *Site) DefaultFuncMap() map[string]any {
	return map[string]any{
		"LeafPages":     s.LeafPages,
		"PagesByDate":   s.GetPagesByDate,
		"PagesByTag":    s.GetPagesByTag,
		"AllTags":       GetAllTags,
		"KeysForTagMap": s.KeysForTagMap,
		"json":          s.Json,
		"HtmlTemplate":  s.RenderHtmlTemplate,
		"TextTemplate":  s.RenderTextTemplate,
		"AllRes": func() []*Resource {
			resources := s.ListResources(
				func(res *Resource) bool {
					return !res.IsParametric
				},
				// sort by reverse date order
				/*sort=*/
				nil, -1, -1)
			sort.Slice(resources, func(idx1, idx2 int) bool {
				res1 := resources[idx1]
				res2 := resources[idx2]
				return res1.CreatedAt.Sub(res2.CreatedAt) > 0
			})
			return resources
		},
	}
}

func (s *Site) LeafPages(hideDrafts bool, orderby string, offset, count any) (out []*Resource) {
	var sortFunc ResourceSortFunc = nil
	if orderby != "" {
		desc := orderby[0] == '-'
		if desc {
			orderby = orderby[1:]
		}
		sortFunc = func(res1, res2 *Resource) bool {
			d1 := res1.Page.(*DefaultPage)
			d2 := res2.Page.(*DefaultPage)
			if d1 == nil || d2 == nil {
				log.Println("D1: ", res1.FullPath)
				log.Println("D2: ", res2.FullPath)
				return false
			}
			sub := 0
			if orderby == "date" {
				sub = int(d1.CreatedAt.Sub(d2.CreatedAt))
			} else if orderby == "title" {
				sub = strings.Compare(d1.Title, d2.Title)
			}
			if desc {
				return sub > 0
			} else {
				return sub < 0
			}
		}
	}
	return s.ListResources(
		func(res *Resource) bool {
			// Leaf pages only - not indexes
			if res.IsParametric || !res.NeedsIndex || res.IsIndex {
				return false
			}

			if hideDrafts {
				draft := res.FrontMatter().Data["draft"]
				if draft == true {
					return false
				}
			}
			return true
			// && (strings.HasSuffix(res.FullPath, ".md") || strings.HasSuffix(res.FullPath, ".mdx"))
		},
		sortFunc,
		gotl.ToInt(offset),
		gotl.ToInt(count))
}

func (s *Site) GetPagesByTag(tag string, hideDrafts bool, desc bool, offset, count any) (out []*Resource) {
	return s.ListResources(
		func(res *Resource) bool {
			if res.IsParametric || !(res.NeedsIndex || res.IsIndex) {
				return false
			}

			if hideDrafts {
				draft := res.FrontMatter().Data["draft"]
				if draft == true {
					return false
				}
			}
			tags := res.Page.(*DefaultPage).Tags
			for _, t := range tags {
				if t == tag {
					return true
				}
				if gotl.Slugify(t) == tag {
					return true
				}
			}

			return false
			// && (strings.HasSuffix(res.FullPath, ".md") || strings.HasSuffix(res.FullPath, ".mdx"))
		},
		func(res1, res2 *Resource) bool {
			d1 := res1.Page.(*DefaultPage)
			d2 := res2.Page.(*DefaultPage)
			if d1 == nil || d2 == nil {
				log.Println("D1: ", res1.FullPath)
				log.Println("D2: ", res2.FullPath)
				return false
			}
			sub := res1.Page.(*DefaultPage).CreatedAt.Sub(res2.Page.(*DefaultPage).CreatedAt)
			if desc {
				return sub > 0
			} else {
				return sub < 0
			}
		},
		gotl.ToInt(offset),
		gotl.ToInt(count))
}

func (s *Site) GetPagesByDate(hideDrafts bool, desc bool, offset, count any) (out []*Resource) {
	return s.ListResources(
		func(res *Resource) bool {
			if res.IsParametric || !(res.NeedsIndex || res.IsIndex) {
				return false
			}

			if hideDrafts {
				draft := res.FrontMatter().Data["draft"]
				if draft == true {
					return false
				}
			}
			return true
			// && (strings.HasSuffix(res.FullPath, ".md") || strings.HasSuffix(res.FullPath, ".mdx"))
		},
		func(res1, res2 *Resource) bool {
			d1 := res1.Page.(*DefaultPage)
			d2 := res2.Page.(*DefaultPage)
			if d1 == nil || d2 == nil {
				log.Println("D1: ", res1.FullPath)
				log.Println("D2: ", res2.FullPath)
				return false
			}
			sub := d1.CreatedAt.Sub(d2.CreatedAt)
			if desc {
				return sub > 0
			} else {
				return sub < 0
			}
		},
		gotl.ToInt(offset), gotl.ToInt(count))
}

func (s *Site) KeysForTagMap(tagmap map[string]int, orderby string) []string {
	out := gfn.MapKeys(tagmap)
	sort.Slice(out, func(i1, i2 int) bool {
		c1 := tagmap[out[i1]]
		c2 := tagmap[out[i2]]
		if c1 == c2 {
			return out[i1] < out[i2]
		}
		return c1 > c2
	})
	return out
}

func GetAllTags(resources []*Resource) (tagCount map[string]int) {
	tagCount = make(map[string]int)
	for _, res := range resources {
		if res.FrontMatter().Data != nil {
			if t, ok := res.FrontMatter().Data["tags"]; ok && t != nil {
				if tags, ok := t.([]any); ok && tags != nil {
					for _, tag := range tags {
						tagCount[tag.(string)] += 1
					}
				}
			}
		}
	}
	return
}

func (s *Site) RenderTextTemplate(templateFile, templateName string, params any) (out string, err error) {
	writer := bytes.NewBufferString("")
	tmpl, err := s.Templates.Loader.Load(templateFile, "")
	if err == nil {
		if tmpl[0].Name == "" {
			tmpl[0].Name = templateName
		}
		err = s.Templates.RenderTextTemplate(writer, tmpl[0], templateName, params, nil)
		out = writer.String()
	} else {
		log.Println("ERR: ", err)
	}
	return
}

func (s *Site) RenderHtmlTemplate(templateFile, templateName string, params any) (out template.HTML, err error) {
	writer := bytes.NewBufferString("")
	tmpl, err := s.Templates.Loader.Load(templateFile, "")
	if err == nil {
		if tmpl[0].Name == "" {
			tmpl[0].Name = templateName
		}
		err = s.Templates.RenderHtmlTemplate(writer, tmpl[0], templateName, params, nil)
		out = template.HTML(writer.String())
	} else {
		log.Println("ERR: ", err)
	}
	return
}

func (s *Site) Json(path string, fieldpath string) (any, error) {
	if path[0] == '/' {
		return nil, fmt.Errorf("Invalid json file: %s.  Cannot start with a /", path)
	}
	fullpath := gut.ExpandUserPath(filepath.Join(s.ContentRoot, path))
	res := s.GetResource(fullpath)
	if res.Ext() != ".json" {
		return nil, fmt.Errorf("Invalid json file: %s, Ext: %s", fullpath, res.Ext())
	}

	data, err := res.ReadAll()
	if err != nil {
		return nil, err
	}
	out, err := gut.JsonDecodeBytes(data)
	if err != nil {
		log.Println("Error Decoding Json: ", path, err)
	}
	return out, err
}
