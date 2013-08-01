// Provides a simple trie-based HTTP router that uses handler functions.
//
// The author had yet to find an HTTP router that had a feature-complexity ratio
// to his taste, so he wrote his own. This HTTP router is trie-based, using each
// path component of a URL as a leaf in the trie. It has additional support for
// wildcard matches (to compose argument lists) and subtree not-found/error 
// handlers.
//
// A major deviation from other APIs is that routes do not use the standard
// http.Handler interface -- the reason behind this is that I prefer to specify
// error handlers as part of the router description, rather than wrapping each
// routed function handler. This requires the route functions to return an error,
// which http.Handler does not cleanly facilitate. The top-level Router struct, 
// however, provides ServeHTTP so it can be used with everyone else's code.
//
// Here's an example router setup:
//
//     rtr := router.NewRouter()
//     rtr.Handle("GET", "/", landingPage)
//     rtr.SetDefault("GET", "/", default404)
//     rtr.SetErrorHandler("GET", "/", defaultError)
//
//     // Here's a case where you really want different error/404 handlers:
//     rtr.SetDefault("GET", "/api/1/", api404)
//     rtr.SetErrorHandler("GET", "/api/1/", apiError)
//     rtr.Handle("GET", "/api/1/post/id/*", api1PostById)
//     rtr.Handle("POST", /api/1/post", api1CreatePost)
//    
//     // etc.
//
// An important note is that this router treats /foo and /foo/ as different routes
// for the purpose of default/error handlers (but not for normal route purposes) --
// 
//     rtr := router.NewRouter()
//     rtr.SetDefault("GET", "/", rt1)     // matches /foo
//     rtr.SetDefault("GET", "/foo", rt2)  // matches /foo/
//
// With the above router, "/foo" hits rt1, while "/foo/" hits rt2. The logic 
// (in MVC terms, at least) is that "/foo" corresponds to the "foo" method of the 
// root controller, whereas "/foo/" is the "index" method of the "foo" controller.
package router

import (
	"net/http"
	"strings"
)

// Route is a type alias for a handler function. 
//
// If the path the route is bound to contains '*' wildcards, args is filled
// in with the values used as wildcards. e.g., if you have the following router:
//
//     rtr := router.NewRouter()
//     rtr.Handle("GET", "/*/foo/*", myRoute)
//
// Then the URL "/a/foo/d" will have args ["a", "d"]. Since "foo" is not a wildcard
// in the route, it is not included.
//
// If a non-nil error is returned, it will be passed to the nearest ErrorHandler.
type Route func(w http.ResponseWriter, r *http.Request, args []string) error

// ErrorHandler is a specialized route that is invoked when a Router returns an error.
type ErrorHandler func(w http.ResponseWriter, r *http.Request, er error)

// Built-in default for when there is no default given.
func nullRoute(w http.ResponseWriter, r *http.Request, args []string) error {
	http.NotFound(w, r)
	return nil
}

// Built-in error handler for when there is none provided.
func nullErrorHandler(w http.ResponseWriter, r *http.Request, er error) {
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}

// Router provides an interface for constructing an routing requests to a trie-based
// routing system.
type Router struct {
	children     map[string]*subrouter
}

// NewRouter constructs a new Router.
func NewRouter() *Router {
	return &Router{
		children:     make(map[string]*subrouter),
	}
}

// Handle registers a new Route corresponding to the provided (method, url) pair.
// If there is already a route registered for the pair, it panics.
func (r *Router) Handle(method, url string, rt Route) {
	sr := r.insertSubrouter(method, url)

	if sr.route != nil {
		panic("router: already exists a route for " + method + " " + url)
	}

	sr.route = rt
}

// SetDefault registers a default Route for all unmatched requests whose prefix
// matches the specified (method, url) tuple. It will not match the specified
// url directly, see the package notes.
//
// If there is already a default handler registered for this (method, url) tuple,
// the function panics.
func (r *Router) SetDefault(method, url string, rt Route) {
	sr := r.insertSubrouter(method, url)

	if sr.defaultRoute != nil {
		panic("router: already exists a default route for " + method + " " + url)
	}

	sr.defaultRoute = rt
}

// SetErrorHandler registers an ErrorHandler for all routes that return an error
// within the (method, url) prefix. It is treated nigh-identically to default
// routes.
func (r *Router) SetErrorHandler(method, url string, rt ErrorHandler) {
	sr := r.insertSubrouter(method, url)

	if sr.errorHandler != nil {
		panic("router: already exists an error handler for " + method + " " + url)
	}

	sr.errorHandler = rt
}

// ServeHTTP fetches the best matching route, invokes it, then calls the best-matching
// error handler if the route returned an error.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	pathParts := strings.Split(req.URL.Path, "/")
	rt, erh, args := r.findRoute(req.Method, pathParts)

	if er := rt(w, req, args); er != nil {
		erh(w, req, er)
	}
}

// Helper function to insert a subrouter entry into the Router's trie.
func (r *Router) insertSubrouter(method string, url string) (sr *subrouter) {
	pathParts := strings.Split(url, "/")
	method = strings.ToLower(method)

	sr, ok := r.children[method]
	if !ok {
		sr = newSubrouter()
		r.children[method] = sr
	}

	for _, pathPart := range pathParts {
		if pathPart == "" {
			continue
		}

		tmp, ok := sr.children[pathPart]
		if !ok {
			tmp = newSubrouter()
			sr.children[pathPart] = tmp
		}

		sr = tmp
	}

	return sr
}

// Helper function that walks the Router's trie, gathering wildcard arguments
// and returning them with the best-matching Route and ErrorHandler.
func (r *Router) findRoute(method string, pathParts []string) (rt Route, erh ErrorHandler, args []string) {
	method = strings.ToLower(method)
	rt = nullRoute
	erh = nullErrorHandler

	sr, ok := r.children[method]
	if !ok {
		return
	}

	// Set the default/error handlers for the root URL, since the loop won't
	// be iterated over.
	if sr.defaultRoute != nil {
		rt = sr.defaultRoute
	}

	if sr.errorHandler != nil {
		erh = sr.errorHandler
	}

	for _, pathPart := range pathParts {
		if sr.defaultRoute != nil {
			rt = sr.defaultRoute
		}

		if sr.errorHandler != nil {
			erh = sr.errorHandler
		}

		// Having this here instead of at the beginning of the loop changes the
		// behavior when the URL has a trailing '/'.
		if pathPart == "" {
			continue
		}

		tmp, ok := sr.children[pathPart]
		if !ok {
			tmp, ok = sr.children["*"]
			if !ok {
				return
			}

			args = append(args, pathPart)
		}

		sr = tmp
	}

	if sr.route != nil {
		rt = sr.route
	}

	return
}

// Node in the Router's trie.
type subrouter struct {
	children map[string]*subrouter
	route Route
	defaultRoute Route
	errorHandler ErrorHandler
}

func newSubrouter() *subrouter {
	return &subrouter{
		children: map[string]*subrouter{},
	}
}
