package internal

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/prologic/twtxt/internal/session"
	log "github.com/sirupsen/logrus"
)

// ReportHandler ...
func (s *Server) ReportHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		nick := strings.TrimSpace(r.FormValue("nick"))
		url := NormalizeURL(r.FormValue("url"))

		if nick == "" || url == "" {
			ctx.Error = true
			ctx.Message = "Both nick and url must be specified"
			s.render("error", w, ctx)
			return
		}

		if r.Method == "GET" {
			ctx.Title = "Report abuse"
			ctx.ReportNick = nick
			ctx.ReportURL = url
			s.render("report", w, ctx)
			return
		}

		nick = strings.TrimSpace(r.FormValue("nick"))
		url = strings.TrimSpace(r.FormValue("url"))

		name := strings.TrimSpace(r.FormValue("name"))
		email := strings.TrimSpace(r.FormValue("email"))
		category := strings.TrimSpace(r.FormValue("category"))
		message := strings.TrimSpace(r.FormValue("message"))

		captchaInput := strings.TrimSpace(r.FormValue("captchaInput"))

		// Get session
		sess := r.Context().Value(session.SessionKey)
		if sess == nil {
			log.Warn("no session found")
			ctx.Error = true
			ctx.Message = fmt.Sprintf("no session found, do you have cookies disabled?")
			s.render("error", w, ctx)
			return
		}

		// Get captcha text from session
		captchaText, isCaptchaTextAvailable := sess.(*session.Session).Get("captchaText")
		if isCaptchaTextAvailable == false {
			log.Warn("no captcha provided")
			ctx.Error = true
			ctx.Message = "no captcha text found"
			s.render("error", w, ctx)
			return
		}

		if captchaInput != captchaText {
			log.Warn("incorrect captcha")
			ctx.Error = true
			ctx.Message = "Unable to match captcha text. Please try again."
			s.render("error", w, ctx)
			return
		}

		if err := SendReportAbuseEmail(s.config, nick, url, name, email, category, message); err != nil {
			log.WithError(err).Errorf("unable to send report email for %s", email)
			ctx.Error = true
			ctx.Message = "Error sending report! Please try again."
			s.render("error", w, ctx)
			return
		}

		ctx.Error = false
		ctx.Message = fmt.Sprintf(
			"Thank you for your report! Pod operator %s will get back to you soon!",
			s.config.AdminName,
		)
		s.render("error", w, ctx)
	}
}
