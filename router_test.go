package router

import (
	"fmt"
	"net/url"
	"net/http"
	"testing"
)

var (
	lastVal int
	lastArgs []string
)

func examineSubrouter(sub *subrouter, prefix string) {
	if sub.route != nil {
		fmt.Printf("%s* has route\n", prefix)
	}

	if sub.defaultRoute != nil {
		fmt.Printf("%s* has default\n", prefix)
	}

	if sub.errorHandler != nil {
		fmt.Printf("%s* has error handler\n", prefix)
	}

	for path, child := range sub.children {
		fmt.Printf("%s/%s\n", prefix, path)
		examineSubrouter(child, prefix + "  ")
	}
}

func examineRouter(rtr *Router) {
	for method, sr := range rtr.children {
		fmt.Printf("%s\n", method)
		fmt.Printf("  /\n")
		examineSubrouter(sr, "    ")
	}
}

func makeRoute(val int) Route {
	return func(w http.ResponseWriter, r *http.Request, args []string) error {
		lastVal = val
		lastArgs = args
		return nil
	}
}

func testRoute(rtr *Router, method, urlStr string) {
	u, er := url.Parse(urlStr)
	if er != nil {
		panic(er)
	}

	rtr.ServeHTTP(nil, &http.Request{
		Method: method,
		URL: u,
	})
}

func TestSimpleRoutes(t *testing.T) {
	rtr := NewRouter()

	rtr.Handle("GET", "/", makeRoute(1))
	rtr.Handle("GET", "/foo", makeRoute(2))
	rtr.Handle("GET", "/*", makeRoute(3))

	testRoute(rtr, "GET", "/")
	if lastVal != 1 {
		t.Errorf("Invalid route for /, got %d", lastVal)
	}

	testRoute(rtr, "GET", "/foo")
	if lastVal != 2 {
		t.Errorf("Invalid route for /foo, got %d", lastVal)
	}

	testRoute(rtr, "GET", "/bar")
	if lastVal != 3 {
		t.Errorf("Invalid route for /bar, got %d", lastVal)

	} else if len(lastArgs) == 0 || lastArgs[0] != "bar" {
		t.Errorf("Invalid args for /bar, got %#v", lastArgs)
	}
}

func TestDefaultRoutes(t *testing.T) {
	rtr := NewRouter()

	rtr.SetDefault("GET", "/", makeRoute(1))
	rtr.SetDefault("GET", "/foo", makeRoute(2))
	rtr.Handle("GET", "/bar", makeRoute(3))
	rtr.Handle("GET", "/foo/bar", makeRoute(4))

	testRoute(rtr, "GET", "/")
	if lastVal != 1 {
		t.Errorf("Invalid route for /, got %d", lastVal)
	}

	testRoute(rtr, "GET", "/foo")
	if lastVal != 1 {
		t.Errorf("Invalid route for /foo, got %d", lastVal)
	}

	testRoute(rtr, "GET", "/foo/")
	if lastVal != 2 {
		t.Errorf("Invalid route for /foo, got %d", lastVal)
	}

	testRoute(rtr, "GET", "/baz")
	if lastVal != 1 {
		t.Errorf("Invalid route for /baz, got %d", lastVal)
	}

	testRoute(rtr, "GET", "/bar")
	if lastVal != 3 {
		t.Errorf("Invalid route for /bar, got %d", lastVal)
	}

	testRoute(rtr, "GET", "/foo/bar")
	if lastVal != 4 {
		t.Errorf("Invalid route for /foo/bar, got %d", lastVal)
	}

	testRoute(rtr, "GET", "/foo/baz")
	if lastVal != 2 {
		t.Errorf("Invalid route for /foo/baz, got %d", lastVal)
	}
}

func TestArgRoutes(t *testing.T) {
	rtr := NewRouter()

	rtr.Handle("GET", "/", makeRoute(1))
	rtr.Handle("GET", "/*", makeRoute(2))
	rtr.Handle("GET", "/*/*", makeRoute(3))
	rtr.Handle("GET", "/foo/*", makeRoute(4))
	rtr.Handle("GET", "/*/foo", makeRoute(5))
	rtr.SetDefault("GET", "/", makeRoute(6))

	testRoute(rtr, "GET", "/")
	if lastVal != 1 {
		t.Errorf("Invalid route for /, got %d", lastVal)
	}

	testRoute(rtr, "GET", "/foo")
	if lastVal != 6 {
		t.Errorf("Invalid route for /foo, got %d", lastVal)
	}

	testRoute(rtr, "GET", "/foo/")
	if lastVal != 6 {
		t.Errorf("Invalid route for /foo/, got %d", lastVal)
	}

	testRoute(rtr, "GET", "/bar")
	if lastVal != 2 {
		t.Errorf("Invalid route for /bar, got %d", lastVal)

	} else if len(lastArgs) != 1 || lastArgs[0] != "bar" {
		t.Errorf("Invalid args for /bar, got %#v", lastArgs)
	}

	testRoute(rtr, "GET", "/foo/bar")
	if lastVal != 4 {
		t.Errorf("Invalid route for /foo/bar, got %d", lastVal)

	} else if len(lastArgs) != 1 || lastArgs[0] != "bar" {
		t.Errorf("Invalid args for /foo/bar, got %#v", lastArgs)
	}

	testRoute(rtr, "GET", "/bar/foo")
	if lastVal != 5 {
		t.Errorf("Invalid route for /bar/foo, got %d", lastVal)

	} else if len(lastArgs) != 1 || lastArgs[0] != "bar" {
		t.Errorf("Invalid args for /bar/foo, got %#v", lastArgs)
	}

	testRoute(rtr, "GET", "/bar/bar")
	if lastVal != 3 {
		t.Errorf("Invalid route for /bar/bar, got %d", lastVal)

	} else if len(lastArgs) != 2 || lastArgs[0] != "bar" || lastArgs[1] != "bar" {
		t.Errorf("Invalid args for /bar/bar, got %#v", lastArgs)
	}
}
