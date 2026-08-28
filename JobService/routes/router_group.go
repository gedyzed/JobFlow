package routes

import (
	"fmt"
	"strings"
	"net/http"
)

type Router struct {
	mux *http.ServeMux
}

func NewRouter() *Router {
	return &Router{mux: http.NewServeMux()}
}

func (r *Router) NewGroup(prefix string, middlewares ...func(http.Handler) http.Handler) *RouteGroup {
	return &RouteGroup{
		prefix:      cleanPrefix(prefix),
		mux:         r.mux,
		middlewares: middlewares,
	}
}

type RouteGroup struct {
	prefix 		    string 
	mux				*http.ServeMux
	middlewares 	[]func(http.Handler) http.Handler
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

func (rg *RouteGroup) HandleFunc(method string, path string, handler http.HandlerFunc) {
	fullPath := rg.prefix + cleanPath(convertPathParams(path))
	pattern := fmt.Sprintf("%s %s", strings.ToUpper(method), fullPath)

	var finalHandler http.Handler = handler
	for i := len(rg.middlewares) - 1; i >= 0; i-- {
		finalHandler = rg.middlewares[i](finalHandler)
	}

	rg.mux.Handle(pattern, finalHandler)
}

// Convenience helper methods
func (g *RouteGroup) GET(path string, handler http.HandlerFunc) {
	g.HandleFunc(http.MethodGet, path, handler)
}

func (g *RouteGroup) POST(path string, handler http.HandlerFunc) {
	g.HandleFunc(http.MethodPost, path, handler)
}

func (g *RouteGroup) PUT(path string, handler http.HandlerFunc) {
	g.HandleFunc(http.MethodPut, path, handler)
}

func (g *RouteGroup) DELETE(path string, handler http.HandlerFunc) {
	g.HandleFunc(http.MethodDelete, path, handler)
}

// URL cleaning utilities
func cleanPrefix(p string) string {
	if p == "" || p == "/" {
		return ""
	}
	return "/" + strings.Trim(p, "/")
}

func cleanPath(p string) string {
	if p == "" || p == "/" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}

func convertPathParams(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") && len(part) > 1 {
			parts[i] = "{" + part[1:] + "}"
		}
	}
	return strings.Join(parts, "/")
}