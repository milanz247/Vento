package vento

import "strings"

// node is a single vertex in a per-method Trie (prefix tree) of path
// segments. A route like "/users/:id/posts/:post_id" decomposes into the
// segment chain "users" -> ":id" -> "posts" -> ":post_id", one node per
// level.
//
// Terminal nodes carry the fully compiled handler chain (global
// middlewares + route middlewares + controller), assembled once at
// registration time. Serving a request therefore never allocates or
// copies a chain - a deliberate zero-allocation optimization.
type node struct {
	path     string        // this segment, e.g. "users" or ":id"
	children []*node       // child segments reachable from this node
	isWild   bool          // true if path is a ":name" parameter segment
	handlers []HandlerFunc // compiled chain; non-nil only on terminal nodes
}

// insert walks/creates the child chain for segments and stores the
// compiled handler chain on the final node.
func (n *node) insert(segments []string, handlers []HandlerFunc) {
	current := n
	for _, segment := range segments {
		var next *node
		for _, child := range current.children {
			if child.path == segment {
				next = child
				break
			}
		}
		if next == nil {
			next = &node{path: segment, isWild: strings.HasPrefix(segment, ":")}
			current.children = append(current.children, next)
		}
		current = next
	}
	current.handlers = handlers
}

// search recursively resolves segments[depth:] beneath n, filling params
// with every wildcard capture along the way. Static children are tried
// before the wildcard child, and the recursion backtracks cleanly - if a
// static branch dead-ends, the wildcard branch is still attempted, so
// /users/me and /users/:id can coexist with the literal route winning.
func (n *node) search(segments []string, depth int, params map[string]string) *node {
	if depth == len(segments) {
		if n.handlers == nil {
			return nil // intermediate node, not a registered route
		}
		return n
	}

	segment := segments[depth]

	for _, child := range n.children {
		if child.isWild || child.path != segment {
			continue
		}
		if found := child.search(segments, depth+1, params); found != nil {
			return found
		}
	}

	for _, child := range n.children {
		if !child.isWild {
			continue
		}
		if found := child.search(segments, depth+1, params); found != nil {
			params[child.path[1:]] = segment
			return found
		}
	}

	return nil
}

// router owns one Trie per HTTP method, allowing GET, POST, etc. to define
// independent, non-conflicting route trees.
type router struct {
	roots map[string]*node
}

func newRouter() *router {
	return &router{roots: make(map[string]*node)}
}

// splitPath breaks a URL path into its non-empty segments, e.g.
// "/users/:id/profile" -> ["users", ":id", "profile"]. Leading, trailing,
// and repeated slashes collapse away since empty parts are skipped.
func splitPath(path string) []string {
	parts := strings.Split(path, "/")
	segments := parts[:0]
	for _, part := range parts {
		if part != "" {
			segments = append(segments, part)
		}
	}
	return segments
}

// addRoute parses path into segments and inserts the pre-compiled handler
// chain into the Trie for method, creating that method's root on first use.
func (r *router) addRoute(method, path string, handlers []HandlerFunc) {
	root, exists := r.roots[method]
	if !exists {
		root = &node{path: "/"}
		r.roots[method] = root
	}
	root.insert(splitPath(path), handlers)
}

// getRoute resolves method+path to a terminal node and the dynamic
// parameters captured on the way (e.g. {"id": "42"} for /users/42 against
// /users/:id). It returns (nil, nil) when nothing matches.
func (r *router) getRoute(method, path string) (*node, map[string]string) {
	root, exists := r.roots[method]
	if !exists {
		return nil, nil
	}

	segments := splitPath(path)
	params := make(map[string]string)

	found := root.search(segments, 0, params)
	if found == nil {
		return nil, nil
	}
	return found, params
}
