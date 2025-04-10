
## Ordering and Dependencies

Currently there is a very nasty bug in s3gen.   When the site is rebuild it walks down and collects all resources
recursively.  And then once fetched, goes on to rendering each one of them.   The rule handler for that resource
is invoked which then renders the resource onto its target (eg .md handled by MDToHtml and so on).

Take the base folder - content.  In this content folder index.html is typically used to render a "list" page of
all posts found on the site.   However this "index" page makes a call to ListResources that lists all resources
and filters out pages that are not leaves.   But whether a page is a leaf or not is only evaluated when it
is "loaded" by the Rule Handler.   This is because the rule handler knows how to consider a resource (ie whether an .md
page is a leaf or not). So we have an issue of dependencies.  Since a set of pages referred to by index.html have not
been rendered they have not yet been loaded - so their leaf flag is not set and as a result the index.html page
will miss out on them.

Fun fact - This worked fine in the blog because the "content/blog" folder appeared before "content/index.html" in the
recursive walk so by the time index.html was reached the blog/* items were loaded.

There are two ways to think of this issue:

1. Should only the rule handler be able to "load" and assign meaning to resources - A related view is who/what should be
   responsible for "loading" a resource.   This is tricky because .md and .html files could be treated differently.
   Even with .html index.html may have a different rule than how-to-make-money.html.
2. How should a resource identify what other resources it depends on - especially dynamically so that the engine can
   take care of order of these?

## Options

### 1. Assign priorities

We could just assign priorities to various resource types, eg our Resource could also have a priority field that
determines when it is processed.  Those with a lower priority number will be rendered before higher priority resources.
This would ensure index.html could be given a higher priority than .md files so all .md resources would get rendered
first and *then* index.html

This is simplest and gives most control.  Our site def could be something like (other complex combos are possible):

```
site = Site {
  ...
  PriorityFunc = func(res *Resource) int {
    if res.FullPath.EndsWith("index.html") {    // ensure index.html is always rendered last
      return 1000
    }
    return 0
  }
}
```

### 2. Make Listing return ALL resources

Today we place a few restrictions on which resources will show up in a listing (eg !IsParameter, NeedsIndex or IsIndex).
And these parameters are only set when a resource is "loaded".   But then itis upto the rendered do a whole bunch of 
checks and filtering on all resources - means more actual code for the user.

TODO - Even though this is clunkier - we still need to think of why the Listing is "auto" filtering by `IsParametric and
(NeedsIndex or IsIndex)`.

3. Centralize Resource Loading

Taking from the above - one issue is with NeedsIndex and IsIndex.   It is set when a resource is "loaded" by the rule
handler.  This is because a single resource can be used to generate multiple outputs.  Eg one x.md can be used to
generate x.html.  However another rule can take a bunch of .md in a directory (including x.md) and create a single
y.html from it.

In the first case x.md needs an index and in the second case it does not.

Worse - in index.html both x.html and y.html may be referred - in which case is it dealing with x.md or y.md?

We can do a couple of things here to handle this:

* Have a single "definition" of an input resource - so x.md  if it needs an index, it will always need an index.
  Problem with this is use in two different rules.   The above case of x.html and y.html wont be reflected.
* Instead of index.html depending on input sources - it can depend on the outputs.  Ie index.html really needs all posts
  to be generated so it can point to their links.  But how can this be specified (are we talking about the same problem
  again?)

### 3. Declare Dependencies

As a resource is processed - we could simply have it "declare" its dependencies, eg:

In index.html:

```
---
Front Matter...
---

{{ $posts = GetAllResources | (FilterByMarkDown or FilterByHtml) }}
{{ DeclareDependencies $posts }}

```

There are a few problems with this:

1. We could already be in the middle of a render making this too late.
2. This still needs template owners to provide custom "filter" functions - especially for dynamic listing pages.
3. And again we risk running the same issue as above


A simpler version of this is to just have us declare the dependencies as a list of regexes of other resources that
*must* be built first. eg:

```
___
a: 1
b: 2
...
Needs: ["regex1", "regex2", "regex3" ....]
---

Rest of content
```

This option is not bad - but still feels like a bit of work.

### 4. Rule based analysis and intermediate targets

Currently our "rules" are arbitrary.  Why not have another rule that creates Tags.json, Posts.json etc - to act as data
sources.  Our tags.html etc would depend on this .json file?   So specifying dependencies is still key (as part of #3)
but they are based on an output target instead of dynamically knowing all posts?

## Recommendation

For now we will go with the easiest option - specifying priorities.  By default, (if a priority function is not
specified) the following will be used (ie reverse order of rendering):

1. 10000: Parametric Pages
2. 5000: index.* or `_index.*`
3. 1000: *.md *.html *.mdx *.htm
4. 0: Everytin else

How does something like "tags.html" here work?  Tags page needs all other leaf pages to be done first.  This would also
need us to add tags.html into the 5000 range.
