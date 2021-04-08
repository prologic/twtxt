package internal

import (
	"fmt"
	"io/fs"

	"github.com/naoina/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"

	"github.com/jointwt/twtxt/internal/langs"
)

type Translator struct {
	Bundle *i18n.Bundle
}

func NewTranslator() (*Translator, error) {
	// lang
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	buf, err := fs.ReadFile(langs.LocaleFS, "active.en.toml")
	if err != nil {
		return nil, fmt.Errorf("error loading en locale: %w", err)
	}
	bundle.MustParseMessageFileBytes(buf, "active.en.toml")

	buf, err = fs.ReadFile(langs.LocaleFS, "active.zh-cn.toml")
	if err != nil {
		return nil, fmt.Errorf("error loading zh-cn locale: %w", err)
	}
	bundle.MustParseMessageFileBytes(buf, "active.zh-cn.toml")

	return &Translator{
		Bundle: bundle,
	}, nil
}

// Translate 翻译
func (t *Translator) Translate(ctx *Context, msgID string, data ...interface{}) string {
	localizer := i18n.NewLocalizer(t.Bundle, ctx.Lang, ctx.AcceptLangs)

	conf := i18n.LocalizeConfig{
		MessageID: msgID,
	}
	if len(data) > 0 {
		conf.TemplateData = data[0]
	}

	return localizer.MustLocalize(&conf)

}

func MustLoadMessageFromFS(b *i18n.Bundle, fsys fs.FS, path string) {
	if _, err := LoadMessageFromFS(b, fsys, path); err != nil {
		panic(err)
	}
}

// LoadMessageFromFileFS is like LoadMessageFile but instead of reading from the
// hosts operating system's file system it reads from the fs file system.
func LoadMessageFromFS(b *i18n.Bundle, fsys fs.FS, path string) (*i18n.MessageFile, error) {
	buf, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, err
	}

	return b.ParseMessageFileBytes(buf, path)
}
