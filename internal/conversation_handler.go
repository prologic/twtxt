package internal

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/securisec/go-keywords"
	log "github.com/sirupsen/logrus"
	"github.com/vcraescu/go-paginator"
	"github.com/vcraescu/go-paginator/adapter"

	"github.com/prologic/twtxt/types"
)

// ConversationHandler ...
func (s *Server) ConversationHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		hash := p.ByName("hash")
		if hash == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		var err error

		twt, ok := s.cache.Lookup(hash)
		if !ok {
			// If the twt is not in the cache look for it in the archive
			if s.archive.Has(hash) {
				twt, err = s.archive.Get(hash)
				if err != nil {
					ctx.Error = true
					ctx.Message = "Error loading twt from archive, please try again"
					s.render("error", w, ctx)
					return
				}
			}
		}

		if twt.IsZero() {
			ctx.Error = true
			ctx.Message = "No matching twt found!"
			s.render("404", w, ctx)
			return
		}

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

		getTweetsByHash := func(hash string, replyTo types.Twt) types.Twts {
			var result types.Twts
			seen := make(map[string]bool)
			// TODO: Improve this by making this an O(1) lookup on the tag
			for _, twt := range s.cache.GetAll() {
				if HasString(UniqStrings(twt.Tags()), hash) && !seen[twt.Hash()] {
					result = append(result, twt)
					seen[twt.Hash()] = true
				}
			}
			if !seen[replyTo.Hash()] {
				result = append(result, replyTo)
			}
			return result
		}

		twts := getTweetsByHash(hash, twt)
		sort.Sort(sort.Reverse(twts))

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

		if r.Method == http.MethodHead {
			defer r.Body.Close()
			return
		}

		ctx.Title = fmt.Sprintf("%s @ %s > %s ", who, when, what)
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

		ctx.Reply = fmt.Sprintf("#%s", twt.Hash())
		ctx.Twts = pagedTwts
		ctx.Pager = &pager
		s.render("conversation", w, ctx)
		return
	}
}
