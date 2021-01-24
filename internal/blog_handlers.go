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
	"github.com/jointwt/twtxt/types"
	"github.com/julienschmidt/httprouter"
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
				var tags types.TagList = twt.Tags()
				if HasString(UniqStrings(tags.Tags()), tag) && !seen[twt.Hash()] {
					result = append(result, twt)
					seen[twt.Hash()] = true
				}
			}
			return result
		}

		twts := getTweetsByTag(blogPost.Hash())

		sort.Sort(sort.Reverse(twts))

		extensions := parser.CommonExtensions |
			parser.NoEmptyLineBeforeBlock |
			parser.AutoHeadingIDs |
			parser.HardLineBreak |
			parser.Footnotes

		mdParser := parser.NewWithExtensions(extensions)

		htmlFlags := html.CommonFlags
		opts := html.RendererOptions{
			Flags:     htmlFlags,
			Generator: "",
		}
		renderer := html.NewRenderer(opts)

		html := markdown.ToHTML(blogPost.Bytes(), mdParser, renderer)

		who := fmt.Sprintf("%s %s", blogPost.Author, URLForUser(s.config, blogPost.Author))
		when := blogPost.Created().Format(time.RFC3339)

		var ks []string
		if ks, err = keywords.Extract(blogPost.Content()); err != nil {
			log.WithError(err).Warn("error extracting keywords")
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Last-Modified", blogPost.Modified().Format(http.TimeFormat))
		w.Header().Set(
			"Link",
			fmt.Sprintf(
				`<%s/user/%s/webmention>; rel="webmention"`,
				s.config.BaseURL, blogPost.Author,
			),
		)

		if r.Method == http.MethodHead {
			defer r.Body.Close()
			return
		}

		ctx.Title = fmt.Sprintf(
			"%s @ %s > published Twt Blog %s: %s ",
			who, when,
			blogPost.String(), blogPost.Title,
		)
		ctx.Content = template.HTML(html)
		ctx.Meta = Meta{
			Author:      blogPost.Author,
			Description: blogPost.Title,
			Keywords:    strings.Join(ks, ", "),
		}
		ctx.Links = append(ctx.Links, types.Link{
			Href: fmt.Sprintf("%s/webmention", UserURL(URLForUser(s.config, blogPost.Author))),
			Rel:  "webmention",
		})
		ctx.Alternatives = append(ctx.Alternatives, types.Alternatives{
			types.Alternative{
				Type:  "text/plain",
				Title: fmt.Sprintf("%s's Twtxt Feed", blogPost.Author),
				URL:   URLForUser(s.config, blogPost.Author),
			},
			types.Alternative{
				Type:  "application/atom+xml",
				Title: fmt.Sprintf("%s's Atom Feed", blogPost.Author),
				URL:   fmt.Sprintf("%s/atom.xml", UserURL(URLForUser(s.config, blogPost.Author))),
			},
		}...)

		var pagedTwts types.Twts

		page := SafeParseInt(r.FormValue("p"), 1)
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

// EditBlogHandler ...
func (s *Server) EditBlogHandler() httprouter.Handle {
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

		ctx.Title = fmt.Sprintf("Editing Twt Blog: %s", blogPost.Title)
		ctx.BlogPost = blogPost

		s.render("edit_blogpost", w, ctx)
	}
}

// DeleteBlogHandler ...
func (s *Server) DeleteBlogHandler() httprouter.Handle {
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

		ctx.Title = fmt.Sprintf("Editing Twt Blog: %s", blogPost.Title)
		ctx.BlogPost = blogPost

		s.render("delete_blogpost", w, ctx)
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
			profile = user.Profile(s.config.BaseURL, ctx.User)
		} else if s.db.HasFeed(author) {
			feed, err := s.db.GetFeed(author)
			if err != nil {
				log.WithError(err).Errorf("error loading feed object for %s", author)
				ctx.Error = true
				ctx.Message = "Error loading profile"
				s.render("error", w, ctx)
				return
			}
			profile = feed.Profile(s.config.BaseURL, ctx.User)
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

		blogPosts, err := GetBlogPostsByAuthor(s.config, author)
		if err != nil {
			log.WithError(err).Errorf("error loading blog posts for %s", author)
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Error loading blog posts for %s, Please try again later!", author)
			s.render("error", w, ctx)
			return
		}

		sort.Sort(blogPosts)

		var pagedBlogPosts BlogPosts

		page := SafeParseInt(r.FormValue("p"), 1)
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
		defer r.Body.Close()

		// Extract form fields
		postas := strings.ToLower(strings.TrimSpace(r.FormValue("postas")))
		title := strings.TrimSpace(r.FormValue("title"))
		text := r.FormValue("text")

		// Cleanup the text and convert DOS line ending \r\n to UNIX \n
		text = strings.TrimSpace(text)
		text = strings.ReplaceAll(text, "\r\n", "\n")
		text = strings.ReplaceAll(text, "\n", "\u2028")

		if text == "" {
			ctx.Error = true
			ctx.Message = "No content provided!"
			s.render("error", w, ctx)
			return
		}

		// Expand Mentions and Tags
		twt := types.MakeTwt(types.NilTwt.Twter(), time.Time{}, text)
		twt.ExpandLinks(s.config, NewFeedLookup(s.config, s.db, ctx.User))
		text = twt.FormatText(types.MarkdownFmt, s.config)

		hash := r.FormValue("hash")
		if hash != "" {
			blogPost, ok := s.blogs.Get(hash)
			if !ok {
				log.WithField("hash", hash).Warn("invalid blog hash or blog not found")
				ctx.Error = true
				ctx.Message = "Invalid blog or blog not found"
				s.render("error", w, ctx)
				return
			}
			blogPost.Reset()

			if _, err := blogPost.WriteString(text); err != nil {
				log.WithError(err).Error("error writing blog post content")
				ctx.Error = true
				ctx.Message = "An error occurred updating blog post"
				s.render("error", w, ctx)
				return
			}

			if err := blogPost.Save(s.config); err != nil {
				log.WithError(err).Error("error saving blog post")
				ctx.Error = true
				ctx.Message = "An error occurred updating blog post"
				s.render("error", w, ctx)
				return
			}
			http.Redirect(w, r, blogPost.URL(s.config.BaseURL), http.StatusFound)
			return
		}

		if title == "" {
			ctx.Error = true
			ctx.Message = "No title provided!"
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

		var blogPost *BlogPost

		switch postas {
		case "", user.Username:
			blogPost, err = WriteBlog(s.config, user, title, text)
		default:
			if user.OwnsFeed(postas) {
				blogPost, err = WriteBlogAs(s.config, postas, title, text)
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

		twtText := fmt.Sprintf("[%s](%s)", blogPost.Title, blogPost.URL(s.config.BaseURL))

		if postas == "" || postas == user.Username {
			_, err = AppendTwt(s.config, s.db, user, twtText)
		} else {
			_, err = AppendSpecial(s.config, s.db, postas, twtText)
		}
		if err != nil {
			log.WithError(err).Error("error posting blog post twt")
			ctx.Error = true
			ctx.Message = "Error posting announcement twt for new blog post"
			s.render("error", w, ctx)
			return
		}

		// Update blogs cache
		s.blogs.Add(blogPost)

		// Update user's own timeline with their own new post.
		s.cache.FetchTwts(s.config, s.archive, user.Source(), nil)

		// Re-populate/Warm cache with local twts for this pod
		s.cache.GetByPrefix(s.config.BaseURL, true)

		http.Redirect(w, r, "/", http.StatusFound)
	}
}
