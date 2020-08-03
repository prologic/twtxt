package twtxt

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/julienschmidt/httprouter"
)

func TestHandle(t *testing.T) {
	router := NewRouter()

	h := func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
		w.WriteHeader(http.StatusTeapot)
	}
	router.Handle("GET", "/", h)

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, r)
	if w.Code != http.StatusTeapot {
		t.Error("Test Handle failed")
	}
}

func TestHandler(t *testing.T) {
	router := NewRouter()

	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	router.Handler("GET", "/", h)

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, r)
	if w.Code != http.StatusTeapot {
		t.Error("Test Handler failed")
	}
}

func TestHandlerFunc(t *testing.T) {
	router := NewRouter()

	h := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}
	router.HandlerFunc("GET", "/", h)

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, r)
	if w.Code != http.StatusTeapot {
		t.Error("Test HandlerFunc failed")
	}
}

func TestMethod(t *testing.T) {
	router := NewRouter()

	router.DELETE("/delete", func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
		w.WriteHeader(http.StatusTeapot)
	})

	router.GET("/get", func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
		w.WriteHeader(http.StatusTeapot)
	})

	router.HEAD("/head", func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
		w.WriteHeader(http.StatusTeapot)
	})

	router.OPTIONS("/options", func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
		w.WriteHeader(http.StatusTeapot)
	})

	router.PATCH("/patch", func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
		w.WriteHeader(http.StatusTeapot)
	})

	router.POST("/post", func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
		w.WriteHeader(http.StatusTeapot)
	})

	router.PUT("/put", func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
		w.WriteHeader(http.StatusTeapot)
	})

	samples := map[string]string{
		"DELETE":  "/delete",
		"GET":     "/get",
		"HEAD":    "/head",
		"OPTIONS": "/options",
		"PATCH":   "/patch",
		"POST":    "/post",
		"PUT":     "/put",
	}
	for method, path := range samples {
		r := httptest.NewRequest(method, path, nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, r)
		if w.Code != http.StatusTeapot {
			t.Errorf("Path %s not registered", path)
		}
	}
}

func TestGroup(t *testing.T) {
	router := NewRouter()
	foo := router.Group("/foo")
	bar := router.Group("/bar")
	baz := foo.Group("/baz")

	foo.HandlerFunc("GET", "", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	foo.HandlerFunc("GET", "/group", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	bar.HandlerFunc("GET", "/group", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	baz.HandlerFunc("GET", "/group", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	samples := []string{"/foo", "/foo/group", "/foo/baz/group", "/bar/group"}

	for _, path := range samples {
		r := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, r)
		if w.Code != http.StatusTeapot {
			t.Errorf("Grouped path %s not registered", path)
		}
	}
}

func TestMiddleware(t *testing.T) {
	var use, group bool

	router := NewRouter().Use(func(next httprouter.Handle) httprouter.Handle {
		return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
			use = true
			next(w, r, ps)
		}
	})

	foo := router.Group("/foo", func(next httprouter.Handle) httprouter.Handle {
		return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
			group = true
			next(w, r, ps)
		}
	})

	foo.HandlerFunc("GET", "/bar", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	r := httptest.NewRequest("GET", "/foo/bar", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if !use {
		t.Error("Middleware registered by Use() under \"/\" not touched")
	}
	if !group {
		t.Error("Middleware registered by Group() under \"/foo\" not touched")
	}
}

func TestStatic(t *testing.T) {
	files := []string{"temp_1", "temp_2"}
	strs := []string{"test content", "static contents"}

	for i := range files {
		f, _ := os.Create(files[i])
		defer os.Remove(files[i])

		f.WriteString(strs[i])
		f.Sync()
		f.Close()
	}

	pwd, _ := os.Getwd()
	router := NewRouter()
	router.Static("/*filepath", pwd)

	for i := range files {
		r := httptest.NewRequest("GET", "/"+files[i], nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		body := w.Result().Body
		defer body.Close()

		file, _ := ioutil.ReadAll(body)
		if string(file) != strs[i] {
			t.Error("Test Static failed")
		}
	}
}

func TestFile(t *testing.T) {
	str := "test_content"

	f, _ := os.Create("temp_file")
	defer os.Remove("temp_file")

	f.WriteString(str)
	f.Sync()
	f.Close()

	router := NewRouter()
	router.File("/file", "temp_file")

	r := httptest.NewRequest("GET", "/file", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	body := w.Result().Body
	defer body.Close()

	file, _ := ioutil.ReadAll(body)
	if string(file) != str {
		t.Error("Test File failed")
	}
}
