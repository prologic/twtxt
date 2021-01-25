package internal

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/renstrom/shortuuid"
	log "github.com/sirupsen/logrus"
)

// ManagePodHandler ...
func (s *Server) ManagePodHandler() httprouter.Handle {
	isAdminUser := IsAdminUserFactory(s.config)

	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		if !isAdminUser(ctx.User) {
			ctx.Error = true
			ctx.Message = "You are not a Pod Owner!"
			s.render("403", w, ctx)
			return
		}

		if r.Method == "GET" {
			s.render("managePod", w, ctx)
			return
		}

		name := strings.TrimSpace(r.FormValue("podName"))
		logo := strings.TrimSpace(r.FormValue("podLogo"))
		description := strings.TrimSpace(r.FormValue("podDescription"))
		maxTwtLength := SafeParseInt(r.FormValue("maxTwtLength"), s.config.MaxTwtLength)
		openProfiles := r.FormValue("enableOpenProfiles") == "on"
		openRegistrations := r.FormValue("enableOpenRegistrations") == "on"

		// Clean lines from DOS (\r\n) to UNIX (\n)
		logo = strings.ReplaceAll(logo, "\r\n", "\n")

		// Update pod name
		if name != "" {
			s.config.Name = name
		} else {
			ctx.Error = true
			ctx.Message = "Pod name not specified"
			s.render("error", w, ctx)
			return
		}

		// Update pod logo
		if logo != "" {
			s.config.Logo = logo
		} else {
			ctx.Error = true
			ctx.Message = "Pod logo not provided"
			s.render("error", w, ctx)
			return
		}

		// Update pod description
		if description != "" {
			s.config.Description = description
		} else {
			ctx.Error = true
			ctx.Message = "Pod description not provided"
			s.render("error", w, ctx)
			return
		}

		// Update twt length
		s.config.MaxTwtLength = maxTwtLength
		// Update open profiles
		s.config.OpenProfiles = openProfiles
		// Update open registrations
		s.config.OpenRegistrations = openRegistrations

		// Save config file
		if err := s.config.Settings().Save(filepath.Join(s.config.Data, "settings.yaml")); err != nil {
			log.WithError(err).Error("error saving config")
			os.Exit(1)
		}

		ctx.Error = false
		ctx.Message = "Pod updated successfully"
		s.render("error", w, ctx)
	}
}

// ManageUsersHandler ...
func (s *Server) ManageUsersHandler() httprouter.Handle {
	isAdminUser := IsAdminUserFactory(s.config)

	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		if !isAdminUser(ctx.User) {
			ctx.Error = true
			ctx.Message = "You are not a Pod Owner!"
			s.render("403", w, ctx)
			return
		}

		s.render("manageUsers", w, ctx)

	}
}

// AddUserHandler ...
func (s *Server) AddUserHandler() httprouter.Handle {
	isAdminUser := IsAdminUserFactory(s.config)

	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		if !isAdminUser(ctx.User) {
			ctx.Error = true
			ctx.Message = "You are not a Pod Owner!"
			s.render("403", w, ctx)
			return
		}

		username := NormalizeUsername(r.FormValue("username"))
		// XXX: We DO NOT store this! (EVER)
		email := strings.TrimSpace(r.FormValue("email"))

		// Random password -- User is expected to user "Password Reset"
		password := shortuuid.New()

		if err := ValidateUsername(username); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Username validation failed: %s", err.Error())
			s.render("error", w, ctx)
			return
		}

		if s.db.HasUser(username) || s.db.HasFeed(username) {
			ctx.Error = true
			ctx.Message = "User or Feed with that name already exists! Please pick another!"
			s.render("error", w, ctx)
			return
		}

		p := filepath.Join(s.config.Data, feedsDir)
		if err := os.MkdirAll(p, 0755); err != nil {
			log.WithError(err).Error("error creating feeds directory")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		fn := filepath.Join(p, username)
		if _, err := os.Stat(fn); err == nil {
			ctx.Error = true
			ctx.Message = "Deleted user with that username already exists! Please pick another!"
			s.render("error", w, ctx)
			return
		}

		if err := ioutil.WriteFile(fn, []byte{}, 0644); err != nil {
			log.WithError(err).Error("error creating new user feed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		hash, err := s.pm.CreatePassword(password)
		if err != nil {
			log.WithError(err).Error("error creating password hash")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		recoveryHash := fmt.Sprintf("email:%s", FastHash(email))

		user := NewUser()
		user.Username = username
		user.Recovery = recoveryHash
		user.Password = hash
		user.URL = URLForUser(s.config, username)
		user.CreatedAt = time.Now()

		if err := s.db.SetUser(username, user); err != nil {
			log.WithError(err).Error("error saving user object for new user")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		ctx.Error = false
		ctx.Message = "User successfully created"
		s.render("error", w, ctx)
	}
}

// DelUserHandler ...
func (s *Server) DelUserHandler() httprouter.Handle {
	isAdminUser := IsAdminUserFactory(s.config)

	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		if !isAdminUser(ctx.User) {
			ctx.Error = true
			ctx.Message = "You are not a Pod Owner!"
			s.render("403", w, ctx)
			return
		}

		username := NormalizeUsername(r.FormValue("username"))

		user, err := s.db.GetUser(username)
		if err != nil {
			log.WithError(err).Errorf("error loading user object for %s", username)
			ctx.Error = true
			ctx.Message = "Error deleting account"
			s.render("error", w, ctx)
			return
		}

		// Get all user feeds
		feeds, err := s.db.GetAllFeeds()
		if err != nil {
			ctx.Error = true
			ctx.Message = "An error occured whilst deleting your account"
			s.render("error", w, ctx)
			return
		}

		for _, feed := range feeds {
			// Get user's owned feeds
			if user.OwnsFeed(feed.Name) {
				// Get twts in a feed
				nick := feed.Name
				if nick != "" {
					if s.db.HasFeed(nick) {
						// Fetch feed twts
						twts, err := GetAllTwts(s.config, nick)
						if err != nil {
							ctx.Error = true
							ctx.Message = "An error occured whilst deleting your account"
							s.render("error", w, ctx)
							return
						}

						// Parse twts to search and remove uploaded media
						for _, twt := range twts {
							// Delete archived twts
							if err := s.archive.Del(twt.Hash()); err != nil {
								ctx.Error = true
								ctx.Message = "An error occured whilst deleting your account"
								s.render("error", w, ctx)
								return
							}

							mediaPaths := GetMediaNamesFromText(fmt.Sprintf("%t", twt))

							// Remove all uploaded media in a twt
							for _, mediaPath := range mediaPaths {
								// Delete .png
								fn := filepath.Join(s.config.Data, mediaDir, fmt.Sprintf("%s.png", mediaPath))
								if FileExists(fn) {
									if err := os.Remove(fn); err != nil {
										ctx.Error = true
										ctx.Message = "An error occured whilst deleting your account"
										s.render("error", w, ctx)
										return
									}
								}

								// Delete .webp
								fn = filepath.Join(s.config.Data, mediaDir, fmt.Sprintf("%s.webp", mediaPath))
								if FileExists(fn) {
									if err := os.Remove(fn); err != nil {
										ctx.Error = true
										ctx.Message = "An error occured whilst deleting your account"
										s.render("error", w, ctx)
										return
									}
								}
							}
						}
					}
				}

				// Delete feed
				if err := s.db.DelFeed(nick); err != nil {
					ctx.Error = true
					ctx.Message = "An error occured whilst deleting your account"
					s.render("error", w, ctx)
					return
				}

				// Delete feeds's twtxt.txt
				fn := filepath.Join(s.config.Data, feedsDir, nick)
				if FileExists(fn) {
					if err := os.Remove(fn); err != nil {
						log.WithError(err).Error("error removing feed")
						ctx.Error = true
						ctx.Message = "An error occured whilst deleting your account"
						s.render("error", w, ctx)
					}
				}

				// Delete feed from cache
				s.cache.Delete(feed.Source())
			}
		}

		// Get user's primary feed twts
		twts, err := GetAllTwts(s.config, user.Username)
		if err != nil {
			ctx.Error = true
			ctx.Message = "An error occured whilst deleting your account"
			s.render("error", w, ctx)
			return
		}

		// Parse twts to search and remove primary feed uploaded media
		for _, twt := range twts {
			// Delete archived twts
			if err := s.archive.Del(twt.Hash()); err != nil {
				ctx.Error = true
				ctx.Message = "An error occured whilst deleting your account"
				s.render("error", w, ctx)
				return
			}

			mediaPaths := GetMediaNamesFromText(fmt.Sprintf("%t", twt))

			// Remove all uploaded media in a twt
			for _, mediaPath := range mediaPaths {
				// Delete .png
				fn := filepath.Join(s.config.Data, mediaDir, fmt.Sprintf("%s.png", mediaPath))
				if FileExists(fn) {
					if err := os.Remove(fn); err != nil {
						log.WithError(err).Error("error removing media")
						ctx.Error = true
						ctx.Message = "An error occured whilst deleting your account"
						s.render("error", w, ctx)
					}
				}

				// Delete .webp
				fn = filepath.Join(s.config.Data, mediaDir, fmt.Sprintf("%s.webp", mediaPath))
				if FileExists(fn) {
					if err := os.Remove(fn); err != nil {
						log.WithError(err).Error("error removing media")
						ctx.Error = true
						ctx.Message = "An error occured whilst deleting your account"
						s.render("error", w, ctx)
					}
				}
			}
		}

		// Delete user's primary feed
		if err := s.db.DelFeed(user.Username); err != nil {
			ctx.Error = true
			ctx.Message = "An error occured whilst deleting your account"
			s.render("error", w, ctx)
			return
		}

		// Delete user's twtxt.txt
		fn := filepath.Join(s.config.Data, feedsDir, user.Username)
		if FileExists(fn) {
			if err := os.Remove(fn); err != nil {
				log.WithError(err).Error("error removing user's feed")
				ctx.Error = true
				ctx.Message = "An error occured whilst deleting your account"
				s.render("error", w, ctx)
			}
		}

		// Delete user
		if err := s.db.DelUser(user.Username); err != nil {
			ctx.Error = true
			ctx.Message = "An error occured whilst deleting your account"
			s.render("error", w, ctx)
			return
		}

		// Delete user's feed from cache
		s.cache.Delete(user.Source())

		// Re-populate/Warm cache with local twts for this pod
		s.cache.GetByPrefix(s.config.BaseURL, true)

		s.sm.Delete(w, r)

		ctx.Error = false
		ctx.Message = "Successfully deleted account"
		s.render("error", w, ctx)
	}
}
