package auth

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/prologic/twtxt/session"
)

// Options ...
type Options struct {
	login    string
	register string
}

// NewOptions ...
func NewOptions(login, register string) *Options {
	return &Options{login, register}
}

// Manager ...
type Manager struct {
	options *Options
}

// NewManager ...
func NewManager(options *Options) *Manager {
	return &Manager{options}
}

// MustAuth ...
func (m *Manager) MustAuth(next httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		if sess := r.Context().Value("sesssion"); sess != nil {
			if _, ok := sess.(*session.Session).Get("username"); ok {
				next(w, r, p)
				return
			}
		}

		http.Redirect(w, r, m.options.login, http.StatusFound)
	}
}
