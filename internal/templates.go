package internal

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	rice "github.com/GeertJohan/go.rice"
	"github.com/Masterminds/sprig"
	humanize "github.com/dustin/go-humanize"
	log "github.com/sirupsen/logrus"
)

const (
	templatesPath    = "templates"
	baseTemplate     = "base.html"
	partialsTemplate = "_partials.html"
	baseName         = "base"
)
var _ = templatesPath // dead code?
type TemplateManager struct {
	sync.RWMutex

	debug     bool
	templates map[string]*template.Template
	funcMap   template.FuncMap
}

func NewTemplateManager(conf *Config, blogs *BlogsCache, cache *Cache) (*TemplateManager, error) {
	templates := make(map[string]*template.Template)

	funcMap := sprig.FuncMap()

	funcMap["time"] = humanize.Time
	funcMap["hostnameFromURL"] = HostnameFromURL
	funcMap["prettyURL"] = PrettyURL
	funcMap["isLocalURL"] = IsLocalURLFactory(conf)
	funcMap["formatTwt"] = FormatTwtFactory(conf)
	funcMap["unparseTwt"] = UnparseTwtFactory(conf)
	funcMap["formatForDateTime"] = FormatForDateTime
	funcMap["urlForBlog"] = URLForBlogFactory(conf, blogs)
	funcMap["urlForConv"] = URLForConvFactory(conf, cache)
	funcMap["isAdminUser"] = IsAdminUserFactory(conf)

	m := &TemplateManager{debug: conf.Debug, templates: templates, funcMap: funcMap}

	if err := m.LoadTemplates(); err != nil {
		log.WithError(err).Error("error loading templates")
		return nil, fmt.Errorf("error loading templates: %w", err)
	}

	return m, nil
}

func (m *TemplateManager) LoadTemplates() error {
	m.Lock()
	defer m.Unlock()

	box, err := rice.FindBox("templates")
	if err != nil {
		log.WithError(err).Errorf("error finding templates")
		return fmt.Errorf("error finding templates: %w", err)
	}

	err = box.Walk("", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.WithError(err).Error("error talking templates")
			return fmt.Errorf("error walking templates: %w", err)
		}

		fname := info.Name()
		if !info.IsDir() && fname != baseTemplate {
			// Skip _partials.html and also editor swap files, to improve the development
			// cycle. Editors often add suffixes to their swap files, e.g "~" or ".swp"
			// (Vim) and those files are not parsable as templates, causing panics.
			if fname == partialsTemplate || !strings.HasSuffix(fname, ".html") {
				return nil
			}

			name := strings.TrimSuffix(fname, filepath.Ext(fname))
			t := template.New(name).Option("missingkey=zero")
			t.Funcs(m.funcMap)

			template.Must(t.Parse(box.MustString(fname)))
			template.Must(t.Parse(box.MustString(partialsTemplate)))
			template.Must(t.Parse(box.MustString(baseTemplate)))

			m.templates[name] = t
		}
		return nil
	})
	if err != nil {
		log.WithError(err).Error("error loading templates")
		return fmt.Errorf("error loading templates: %w", err)
	}

	return nil
}

func (m *TemplateManager) Add(name string, template *template.Template) {
	m.Lock()
	defer m.Unlock()

	m.templates[name] = template
}

func (m *TemplateManager) Exec(name string, ctx *Context) (io.WriterTo, error) {
	if m.debug {
		log.Debug("reloading templates in debug mode...")
		if err := m.LoadTemplates(); err != nil {
			log.WithError(err).Error("error reloading templates")
			return nil, fmt.Errorf("error reloading templates: %w", err)
		}
	}

	m.RLock()
	template, ok := m.templates[name]
	m.RUnlock()

	if !ok {
		log.WithField("name", name).Errorf("template not found")
		return nil, fmt.Errorf("no such template: %s", name)
	}

	if ctx == nil {
		ctx = &Context{}
	}

	buf := bytes.NewBuffer([]byte{})
	err := template.ExecuteTemplate(buf, baseName, ctx)
	if err != nil {
		log.WithError(err).WithField("name", name).Errorf("error executing template")
		return nil, fmt.Errorf("error executing template %s: %w", name, err)
	}

	return buf, nil
}
