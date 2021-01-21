package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/rickb777/accept"
	log "github.com/sirupsen/logrus"
)

// MediaHandler ...
func (s *Server) MediaHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		name := p.ByName("name")
		if name == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		var fn string

		switch filepath.Ext(name) {
		case ".png":
			metrics.Counter("media", "old_media").Inc()
			w.Header().Set("Content-Type", "image/png")
			fn = filepath.Join(s.config.Data, mediaDir, name)
		case ".webp":
			w.Header().Set("Content-Type", "image/webp")
			fn = filepath.Join(s.config.Data, mediaDir, name)
		case ".mp4":
			w.Header().Set("Content-Type", "video/mp4")
			fn = filepath.Join(s.config.Data, mediaDir, name)
		case ".webm":
			w.Header().Set("Content-Type", "video/webm")
			fn = filepath.Join(s.config.Data, mediaDir, name)
		case ".ogg":
			w.Header().Set("Content-Type", "audio/ogg")
			fn = filepath.Join(s.config.Data, mediaDir, name)
		case ".mp3":
			w.Header().Set("Content-Type", "audio/mp3")
			fn = filepath.Join(s.config.Data, mediaDir, name)
		default:
			if accept.PreferredContentTypeLike(r.Header, "image/webp") == "image/webp" {
				w.Header().Set("Content-Type", "image/webp")
				fn = filepath.Join(s.config.Data, mediaDir, fmt.Sprintf("%s.webp", name))
			} else {
				// Support older browsers like IE11 that don't support WebP :/
				metrics.Counter("media", "old_media").Inc()
				w.Header().Set("Content-Type", "image/png")
				fn = filepath.Join(s.config.Data, mediaDir, fmt.Sprintf("%s.png", name))
			}
		}

		if !FileExists(fn) {
			log.Warnf("media not found: %s", name)
			http.Error(w, "Media Not Found", http.StatusNotFound)
			return
		}

		fileInfo, err := os.Stat(fn)
		if err != nil {
			log.WithError(err).Error("error reading media file info")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		etag := fmt.Sprintf("W/\"%s-%s\"", r.RequestURI, fileInfo.ModTime().Format(time.RFC3339))
		if match := r.Header.Get("If-None-Match"); match != "" {
			if strings.Contains(match, etag) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}

		f, err := os.Open(fn)
		if err != nil {
			log.WithError(err).Error("error opening media file")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer f.Close()

		w.Header().Set("Etag", etag)
		w.Header().Set("Cache-Control", "public, max-age=7776000")

		if r.Method == http.MethodHead {
			return
		}

		http.ServeContent(w, r, filepath.Base(fn), fileInfo.ModTime(), f)
	}
}

// UploadMediaHandler ...
func (s *Server) UploadMediaHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		// Limit request body to to abuse
		r.Body = http.MaxBytesReader(w, r.Body, s.config.MaxUploadSize)
		defer r.Body.Close()

		mfile, headers, err := r.FormFile("media_file")
		if err != nil && err != http.ErrMissingFile {
			if err.Error() == "http: request body too large" {
				log.Warnf("request too large for media upload from %s", FormatRequest(r))
				http.Error(w, "Media Upload Too Large", http.StatusRequestEntityTooLarge)
				return
			}
			log.WithError(err).Error("error parsing form file")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if mfile == nil || headers == nil {
			log.Warn("no valid media file uploaded")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		ctype := headers.Header.Get("Content-Type")

		var uri URI

		if strings.HasPrefix(ctype, "image/") {
			fn, err := ReceiveImage(mfile)
			if err != nil {
				log.WithError(err).Error("error writing uploaded image")
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			uuid, err := s.tasks.Dispatch(NewImageTask(s.config, fn))
			if err != nil {
				log.WithError(err).Error("error dispatching image processing task")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			uri.Type = "taskURI"
			uri.Path = URLForTask(s.config.BaseURL, uuid)
		}

		if strings.HasPrefix(ctype, "audio/") {
			fn, err := ReceiveAudio(mfile)
			if err != nil {
				log.WithError(err).Error("error writing uploaded audio")
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			uuid, err := s.tasks.Dispatch(NewAudioTask(s.config, fn))
			if err != nil {
				log.WithError(err).Error("error dispatching audio transcoding task")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			uri.Type = "taskURI"
			uri.Path = URLForTask(s.config.BaseURL, uuid)
		}

		if strings.HasPrefix(ctype, "video/") {
			fn, err := ReceiveVideo(mfile)
			if err != nil {
				log.WithError(err).Error("error writing uploaded video")
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			uuid, err := s.tasks.Dispatch(NewVideoTask(s.config, fn))
			if err != nil {
				log.WithError(err).Error("error dispatching vodeo transcode task")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			uri.Type = "taskURI"
			uri.Path = URLForTask(s.config.BaseURL, uuid)
		}

		if uri.IsZero() {
			log.Warn("no media file provided or unsupported media type")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		data, err := json.Marshal(uri)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if uri.Type == "taskURI" {
			w.WriteHeader(http.StatusAccepted)
		}
		_, _ = w.Write(data)

	}
}
