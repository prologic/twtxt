package internal

import (
	"github.com/naoina/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

type Translator struct {
	Bundle *i18n.Bundle
}

func NewTranslator() *Translator {
	// lang
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	// No need to load active.en.toml since we are providing default translations.
	bundle.MustLoadMessageFile("./internal/langs/active.en.toml")
	bundle.MustLoadMessageFile("./internal/langs/active.zh-cn.toml")
	// bundle.LoadMessageFile("./langs/active.zh-tw.toml")
	return &Translator{
		Bundle: bundle,
	}

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
