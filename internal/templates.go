package internal

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/sprig"
	humanize "github.com/dustin/go-humanize"
	"github.com/jointwt/twtxt/types"
	log "github.com/sirupsen/logrus"
)

const (
	baseTemplate     = "templates/base.html"
	partialsTemplate = "templates/_partials.html"
	baseName         = "base"
)

//go:embed templates/*.html
var templates embed.FS

type TemplateManager struct {
	sync.RWMutex

	debug     bool
	templates map[string]*template.Template
	funcMap   template.FuncMap
}

func NewTemplateManager(conf *Config, translator *Translator, blogs *BlogsCache, cache *Cache, archive Archiver) (*TemplateManager, error) {
	templates := make(map[string]*template.Template)

	funcMap := sprig.FuncMap()

	funcMap["time"] = humanize.Time
	funcMap["hostnameFromURL"] = HostnameFromURL
	funcMap["prettyURL"] = PrettyURL
	funcMap["isLocalURL"] = IsLocalURLFactory(conf)
	funcMap["formatTwt"] = FormatTwtFactory(conf)
	funcMap["formatTwtText"] = func() func(text string) template.HTML {
		fn := FormatTwtFactory(conf)
		return func(text string) template.HTML {
			twt := types.MakeTwt(types.Twter{}, time.Time{}, text)
			return fn(twt)
		}
	}()
	funcMap["unparseTwt"] = UnparseTwtFactory(conf)
	funcMap["formatForDateTime"] = FormatForDateTime
	funcMap["urlForBlog"] = URLForBlogFactory(conf, blogs)
	funcMap["urlForConv"] = URLForConvFactory(conf, cache, archive)
	funcMap["isAdminUser"] = IsAdminUserFactory(conf)
	funcMap["twtType"] = func(twt types.Twt) string { return fmt.Sprintf("%T", twt) }

	funcMap["html"] = func(text string) template.HTML { return template.HTML(text) }
	funcMap["tr"] = func(ctx *Context, msgid string, data ...interface{}) string {
		return translator.Translate(ctx, msgid, data...)
	}

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

	err := fs.WalkDir(templates, "templates", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			log.WithError(err).Error("error walking templates")
			return fmt.Errorf("error walking templates: %w", err)
		}

		fname := info.Name()
		if !info.IsDir() && path != baseTemplate {
			// Skip _partials.html and also editor swap files, to improve the development
			// cycle. Editors often add suffixes to their swap files, e.g "~" or ".swp"
			// (Vim) and those files are not parsable as templates, causing panics.
			if fname == partialsTemplate || !strings.HasSuffix(fname, ".html") {
				return nil
			}

			name := strings.TrimSuffix(fname, filepath.Ext(fname))
			t := template.New(name).Option("missingkey=zero")
			t.Funcs(m.funcMap)

			if f, err := templates.ReadFile(path); err == nil {
				template.Must(t.Parse(string(f)))
			} else {
				return err
			}

			if f, err := templates.ReadFile(partialsTemplate); err == nil {
				template.Must(t.Parse(string(f)))
			} else {
				return err
			}

			if f, err := templates.ReadFile(baseTemplate); err == nil {
				template.Must(t.Parse(string(f)))
			} else {
				return err
			}

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
