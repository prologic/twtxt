package twtxt

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// Router ...
type Router struct {
	httprouter.Router
}

// NewRouter ...
func NewRouter() *Router {
	return &Router{
		httprouter.Router{
			RedirectTrailingSlash:  true,
			RedirectFixedPath:      true,
			HandleMethodNotAllowed: false,
			HandleOPTIONS:          true,
		},
	}
}

// ServeFilesWithCacheControl ...
func (r *Router) ServeFilesWithCacheControl(path string, root http.FileSystem) {
	if len(path) < 10 || path[len(path)-10:] != "/*filepath" {
		panic("path must end with /*filepath in path '" + path + "'")
	}

	fileServer := http.FileServer(root)

	r.GET(path, func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		w.Header().Set("Vary", "Accept-Encoding")
		w.Header().Set("Cache-Control", "public, max-age=7776000")
		req.URL.Path = ps.ByName("filepath")
		fileServer.ServeHTTP(w, req)
	})
}
