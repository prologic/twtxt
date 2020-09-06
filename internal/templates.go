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
	baseTemplate     = "base.html"
	partialsTemplate = "_partials.html"
	baseName         = "base"
)

type Templates struct {
	sync.Mutex

	templates map[string]*template.Template
}

func NewTemplates(conf *Config, blogs *BlogsCache, cache *Cache) (*Templates, error) {
	templates := make(map[string]*template.Template)

	funcMap := sprig.FuncMap()

	funcMap["time"] = humanize.Time
	funcMap["slugify"] = Slugify
	funcMap["prettyURL"] = PrettyURL
	funcMap["isLocalURL"] = IsLocalURLFactory(conf)
	funcMap["formatTwt"] = FormatTwtFactory(conf)
	funcMap["unparseTwt"] = UnparseTwtFactory(conf)
	funcMap["formatForDateTime"] = FormatForDateTime
	funcMap["urlForBlog"] = URLForBlogFactory(conf, blogs)
	funcMap["urlForConv"] = URLForConvFactory(conf, cache)
	funcMap["isLocal"] = IsLocalFactory(conf)
	funcMap["isAdminUser"] = IsAdminUserFactory(conf)

	box, err := rice.FindBox("templates")
	if err != nil {
		log.WithError(err).Errorf("error finding templates")
		return nil, err
	}

	err = box.Walk("", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.WithError(err).Error("error finding templates")
			return err
		}

		if !info.IsDir() && info.Name() != baseTemplate {
			if info.Name() == partialsTemplate {
				return nil
			}

			name := strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))
			t := template.New(name).Option("missingkey=error")
			t.Funcs(funcMap)
			template.Must(t.Parse(box.MustString(info.Name())))
			template.Must(t.Parse(box.MustString(partialsTemplate)))
			template.Must(t.Parse(box.MustString(baseTemplate)))
			templates[name] = t
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &Templates{templates: templates}, nil
}

func (t *Templates) Add(name string, template *template.Template) {
	t.Lock()
	defer t.Unlock()

	t.templates[name] = template
}

func (t *Templates) Exec(name string, ctx *Context) (io.WriterTo, error) {
	t.Lock()
	template, ok := t.templates[name]
	t.Unlock()
	if !ok {
		log.Errorf("template %s not found", name)
		return nil, fmt.Errorf("no such template: %s", name)
	}

	if ctx == nil {
		ctx = &Context{}
	}

	buf := bytes.NewBuffer([]byte{})
	err := template.ExecuteTemplate(buf, baseName, ctx)
	if err != nil {
		log.WithError(err).Errorf("error parsing template %s: %s", name, err)
		return nil, err
	}

	return buf, nil
}
