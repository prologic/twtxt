package internal

import (
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

const robotsTpl = `User-Agent: *
Disallow: /
Allow: /
Allow: /twt
Allow: /user
Allow: /feed
Allow: /about
Allow: /help
Allow: /blogs
Allow: /privacy
Allow: /support
Allow: /search
Allow: /external
Allow: /atom.xml
Allow: /media
`

// RobotsHandler ...
func (s *Server) RobotsHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		text, err := RenderPlainText(robotsTpl, ctx)
		if err != nil {
			log.WithError(err).Errorf("error rendering robots.txt")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(text)))

		if r.Method == http.MethodHead {
			return
		}

		_, _ = w.Write([]byte(text))
	}
}
