package internal

import (
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/julienschmidt/httprouter"
	"github.com/prologic/twtxt/types"
	"github.com/securisec/go-keywords"
	log "github.com/sirupsen/logrus"
	"github.com/vcraescu/go-paginator"
	"github.com/vcraescu/go-paginator/adapter"
)

// BlogHandler ...
func (s *Server) BlogHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		blogPost, err := BlogPostFromParams(s.config, p)
		if err != nil {
			log.WithError(err).Error("error loading blog post")
			ctx.Error = true
			ctx.Message = "Error loading blog post! Please contact support."
			s.render("error", w, ctx)
			return
		}

		getTweetsByTag := func(tag string) types.Twts {
			var result types.Twts
			seen := make(map[string]bool)
			// TODO: Improve this by making this an O(1) lookup on the tag
			for _, twt := range s.cache.GetAll() {
				if HasString(UniqStrings(twt.Tags()), tag) && !seen[twt.Hash()] {
					result = append(result, twt)
					seen[twt.Hash()] = true
				}
			}
			return result
		}

		twts := getTweetsByTag(blogPost.Hash())

		sort.Sort(sort.Reverse(twts))

		// If the twt is not in the cache look for it in the archive
		if len(twts) == 0 {
			if s.archive.Has(blogPost.Twt) {
				twt, err := s.archive.Get(blogPost.Twt)
				if err != nil {
					ctx.Error = true
					ctx.Message = fmt.Sprintf(
						"Error loading associated twt for blog post %s from archive",
						blogPost,
					)
					s.render("error", w, ctx)
					return
				}

				twts = append(twts, twt)
			}
		}

		if len(twts) == 0 {
			ctx.Error = true
			ctx.Message = fmt.Sprintf(
				"No associated twt found for blog post %s",
				blogPost,
			)
			s.render("404", w, ctx)
			return
		}

		extensions := parser.CommonExtensions | parser.AutoHeadingIDs
		mdParser := parser.NewWithExtensions(extensions)

		htmlFlags := html.CommonFlags
		opts := html.RendererOptions{
			Flags:     htmlFlags,
			Generator: "",
		}
		renderer := html.NewRenderer(opts)

		html := markdown.ToHTML(blogPost.Bytes(), mdParser, renderer)

		twt := twts[0]
		who := fmt.Sprintf("%s %s", twt.Twter.Nick, twt.Twter.URL)
		when := twt.Created.Format(time.RFC3339)
		what := twt.Text

		var ks []string
		if ks, err = keywords.Extract(what); err != nil {
			log.WithError(err).Warn("error extracting keywords")
		}

		for _, twter := range twt.Mentions() {
			ks = append(ks, twter.Nick)
		}
		ks = append(ks, twt.Tags()...)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Last-Modified", twt.Created.Format(http.TimeFormat))
		if strings.HasPrefix(twt.Twter.URL, s.config.BaseURL) {
			w.Header().Set(
				"Link",
				fmt.Sprintf(
					`<%s/user/%s/webmention>; rel="webmention"`,
					s.config.BaseURL, twt.Twter.Nick,
				),
			)
		}

		if r.Method == http.MethodHead {
			defer r.Body.Close()
			return
		}

		ctx.Title = fmt.Sprintf(
			"%s @ %s > %s: %s ",
			who, when,
			blogPost.String(), blogPost.Title,
		)
		ctx.Content = template.HTML(html)
		ctx.Meta = Meta{
			Author:      who,
			Description: what,
			Keywords:    strings.Join(ks, ", "),
		}
		if strings.HasPrefix(twt.Twter.URL, s.config.BaseURL) {
			ctx.Links = append(ctx.Links, types.Link{
				Href: fmt.Sprintf("%s/webmention", UserURL(twt.Twter.URL)),
				Rel:  "webmention",
			})
			ctx.Alternatives = append(ctx.Alternatives, types.Alternatives{
				types.Alternative{
					Type:  "text/plain",
					Title: fmt.Sprintf("%s's Twtxt Feed", twt.Twter.Nick),
					URL:   twt.Twter.URL,
				},
				types.Alternative{
					Type:  "application/atom+xml",
					Title: fmt.Sprintf("%s's Atom Feed", twt.Twter.Nick),
					URL:   fmt.Sprintf("%s/atom.xml", UserURL(twt.Twter.URL)),
				},
			}...)
		}

		var pagedTwts types.Twts

		page := SafeParseInt(r.FormValue("page"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(twts), s.config.TwtsPerPage)
		pager.SetPage(page)

		if err := pager.Results(&pagedTwts); err != nil {
			ctx.Error = true
			ctx.Message = "An error occurred while loading search results"
			s.render("error", w, ctx)
			return
		}

		ctx.Reply = fmt.Sprintf("#%s", blogPost.Hash())
		ctx.BlogPost = blogPost
		ctx.Twts = pagedTwts
		ctx.Pager = &pager

		s.render("blogpost", w, ctx)
	}
}

// BlogsHandler ...
func (s *Server) BlogsHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		author := NormalizeUsername(p.ByName("author"))
		if author == "" {
			ctx.Error = true
			ctx.Message = "No author specified"
			s.render("error", w, ctx)
			return
		}

		author = NormalizeUsername(author)

		var profile types.Profile

		if s.db.HasUser(author) {
			user, err := s.db.GetUser(author)
			if err != nil {
				log.WithError(err).Errorf("error loading user object for %s", author)
				ctx.Error = true
				ctx.Message = "Error loading profile"
				s.render("error", w, ctx)
				return
			}
			profile = user.Profile(s.config.BaseURL)
		} else if s.db.HasFeed(author) {
			feed, err := s.db.GetFeed(author)
			if err != nil {
				log.WithError(err).Errorf("error loading feed object for %s", author)
				ctx.Error = true
				ctx.Message = "Error loading profile"
				s.render("error", w, ctx)
				return
			}
			profile = feed.Profile(s.config.BaseURL)
		} else {
			ctx.Error = true
			ctx.Message = "No author found by that name"
			s.render("404", w, ctx)
			return
		}

		ctx.Profile = profile

		ctx.Links = append(ctx.Links, types.Link{
			Href: fmt.Sprintf("%s/webmention", UserURL(profile.URL)),
			Rel:  "webmention",
		})
		ctx.Alternatives = append(ctx.Alternatives, types.Alternatives{
			types.Alternative{
				Type:  "text/plain",
				Title: fmt.Sprintf("%s's Twtxt Feed", profile.Username),
				URL:   profile.URL,
			},
			types.Alternative{
				Type:  "application/atom+xml",
				Title: fmt.Sprintf("%s's Atom Feed", profile.Username),
				URL:   fmt.Sprintf("%s/atom.xml", UserURL(profile.URL)),
			},
		}...)

		blogPosts, err := GetBlogPosts(s.config, author)
		if err != nil {
			log.WithError(err).Errorf("error loading blog posts for %s", author)
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Error loading blog posts for %s, Please try again later!", author)
			s.render("error", w, ctx)
			return
		}

		sort.Sort(blogPosts)

		var pagedBlogPosts BlogPosts

		page := SafeParseInt(r.FormValue("page"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(blogPosts), s.config.TwtsPerPage)
		pager.SetPage(page)

		if err := pager.Results(&pagedBlogPosts); err != nil {
			log.WithError(err).Error("error sorting and paging twts")
			ctx.Error = true
			ctx.Message = "An error occurred while loading the timeline"
			s.render("error", w, ctx)
			return
		}

		ctx.Title = fmt.Sprintf("%s's Twt Blog Posts", profile.Username)
		ctx.BlogPosts = pagedBlogPosts
		ctx.Pager = &pager

		s.render("blogs", w, ctx)
	}
}

// PublishBlogHandler ...
func (s *Server) PublishBlogHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		// Limit request body to to abuse
		r.Body = http.MaxBytesReader(w, r.Body, s.config.MaxUploadSize)

		postas := strings.ToLower(strings.TrimSpace(r.FormValue("postas")))
		title := strings.TrimSpace(r.FormValue("title"))
		text := r.FormValue("text")
		if text == "" {
			ctx.Error = true
			ctx.Message = "No content provided!"
			s.render("error", w, ctx)
			return
		}

		user, err := s.db.GetUser(ctx.Username)
		if err != nil {
			log.WithError(err).Errorf("error loading user object for %s", ctx.Username)
			ctx.Error = true
			ctx.Message = "Error publishing blog post"
			s.render("error", w, ctx)
			return
		}

		var b *BlogPost

		switch postas {
		case "", user.Username:
			b, err = WriteBlog(s.config, user, title, text)
		default:
			if user.OwnsFeed(postas) {
				b, err = WriteBlogAs(s.config, postas, title, text)
			} else {
				err = ErrFeedImposter
			}
		}

		if err != nil {
			log.WithError(err).Error("error publishing blog post")
			ctx.Error = true
			ctx.Message = "Error publishing blog post"
			s.render("error", w, ctx)
			return
		}

		summary := fmt.Sprintf(
			"(#%s) New Blog Post [%s](%s) by @%s 📝",
			b.Hash(), b.Title, b.URL(s.config.BaseURL), b.Author,
		)

		var twt types.Twt

		if postas == "" || postas == user.Username {
			twt, err = AppendTwt(s.config, s.db, user, summary)
		} else {
			twt, err = AppendSpecial(s.config, s.db, postas, summary)
		}

		if err != nil {
			log.WithError(err).Error("error posting blog post twt")
			ctx.Error = true
			ctx.Message = "Error posting announcement twt for new blog post"
			s.render("error", w, ctx)
			return
		}

		b.Twt = twt.Hash()
		if err := b.Save(s.config); err != nil {
			log.WithError(err).Error("error persisting twt metdata for blog post")
			ctx.Error = true
			ctx.Message = "Error recording twt for new blog post"
			s.render("error", w, ctx)
			return
		}

		// Update user's own timeline with their own new post.
		s.cache.FetchTwts(s.config, s.archive, user.Source())

		// Re-populate/Warm cache with local twts for this pod
		s.cache.GetByPrefix(s.config.BaseURL, true)

		http.Redirect(w, r, RedirectURL(r, s.config, "/"), http.StatusFound)
	}
}
