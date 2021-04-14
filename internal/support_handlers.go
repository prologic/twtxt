package internal

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/jointwt/twtxt/internal/session"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"github.com/steambap/captcha"
)

// CaptchaHandler ...
func (s *Server) CaptchaHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		img, err := captcha.NewMathExpr(150, 50)
		if err != nil {
			log.WithError(err).Errorf("unable to get generate captcha image")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Save captcha text in session
		sess := r.Context().Value(session.SessionKey)
		if sess == nil {
			log.Warn("no session found")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		_ = sess.(*session.Session).Set("captchaText", img.Text)

		w.Header().Set("Content-Type", "image/png")
		if err := img.WriteImage(w); err != nil {
			log.WithError(err).Errorf("error sending captcha image response")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
}

// SupportHandler ...
func (s *Server) SupportHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		if r.Method == "GET" {
			ctx.Title = s.tr(ctx, "PageSupportTitle")
			s.render("support", w, ctx)
			return
		}

		name := strings.TrimSpace(r.FormValue("name"))
		email := strings.TrimSpace(r.FormValue("email"))
		subject := strings.TrimSpace(r.FormValue("subject"))
		message := strings.TrimSpace(r.FormValue("message"))

		captchaInput := strings.TrimSpace(r.FormValue("captchaInput"))

		// Get session
		sess := r.Context().Value(session.SessionKey)
		if sess == nil {
			log.Warn("no session found")
			ctx.Error = true
			ctx.Message = "no session found, do you have cookies disabled?"
			s.render("error", w, ctx)
			return
		}

		// Get captcha text from session
		captchaText, isCaptchaTextAvailable := sess.(*session.Session).Get("captchaText")
		if !isCaptchaTextAvailable {
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

		if err := SendSupportRequestEmail(s.config, name, email, subject, message); err != nil {
			log.WithError(err).Errorf("unable to send support email for %s", email)
			ctx.Error = true
			ctx.Message = "Error sending support message! Please try again."
			s.render("error", w, ctx)
			return
		}

		log.Infof("support message email sent for %s", email)

		ctx.Error = false
		ctx.Message = fmt.Sprintf(
			"Thank you for your message! Pod operator %s will get back to you soon!",
			s.config.AdminName,
		)
		s.render("error", w, ctx)
	}
}

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
			ctx.Message = "no session found, do you have cookies disabled?"
			s.render("error", w, ctx)
			return
		}

		// Get captcha text from session
		captchaText, isCaptchaTextAvailable := sess.(*session.Session).Get("captchaText")
		if !isCaptchaTextAvailable {
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
