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
	// English
	buf, err := fs.ReadFile(langs.LocaleFS, "active.en.toml")
	if err != nil {
		return nil, fmt.Errorf("error loading en locale: %w", err)
	}
	bundle.MustParseMessageFileBytes(buf, "active.en.toml")
	// Simplified Chinese
	buf, err = fs.ReadFile(langs.LocaleFS, "active.zh-cn.toml")
	if err != nil {
		return nil, fmt.Errorf("error loading zh-cn locale: %w", err)
	}
	bundle.MustParseMessageFileBytes(buf, "active.zh-cn.toml")
	// Traditional Chinese
	buf, err = fs.ReadFile(langs.LocaleFS, "active.zh-tw.toml")
	if err != nil {
		return nil, fmt.Errorf("error loading zh-tw locale: %w", err)
	}
	bundle.MustParseMessageFileBytes(buf, "active.zh-tw.toml")

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
