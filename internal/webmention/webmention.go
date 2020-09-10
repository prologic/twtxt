// webmention project webmention.go
package webmention

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/andyleap/microformats"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type WebMention struct {
	inbox       chan *mention
	outbox      chan *mention
	inboxTimer  *time.Timer
	outboxTimer *time.Timer
	Mention     func(source, target *url.URL, sourceData *microformats.Data) error
}

func New() *WebMention {
	wm := &WebMention{
		inbox:  make(chan *mention, 100),
		outbox: make(chan *mention, 100),
	}
	wm.inboxTimer = time.NewTimer(5 * time.Second)
	wm.outboxTimer = time.NewTimer(5 * time.Second)
	go func() {
		for _ = range wm.inboxTimer.C {
			wm.processInbox()
		}
	}()
	go func() {
		for _ = range wm.outboxTimer.C {
			wm.processOutbox()
		}
	}()
	return wm
}

type mention struct {
	source *url.URL
	target *url.URL
}

func (wm *WebMention) GetTargetEndpoint(target *url.URL) (*url.URL, error) {
	res, err := http.Get(target.String())
	if err != nil {
		log.WithError(err).Error("error getting target endpoint")
		return nil, err
	}
	defer res.Body.Close()

	links := GetHeaderLinks(res.Header["Link"])
	for _, link := range links {
		for _, rel := range link.Params["rel"] {
			if rel == "webmention" || rel == "http://webmention.org" {
				return link.URL, nil
			}
		}
	}

	parser := microformats.New()
	mf2data := parser.Parse(res.Body, target)

	for _, link := range mf2data.Rels["webmention"] {
		wmurl, err := url.Parse(link)
		if err != nil {
			log.WithError(err).Warn("error parsing webmention link")
			continue
		}
		return wmurl, nil
	}

	return nil, nil
}

func (wm *WebMention) SendNotification(target *url.URL, source *url.URL) {
	wm.outbox <- &mention{source, target}
}

func (wm *WebMention) WebMentionEndpoint(w http.ResponseWriter, r *http.Request) {
	source := r.FormValue("source")
	target := r.FormValue("target")
	if source != "" && target != "" {
		sourceurl, _ := url.Parse(source)
		targeturl, _ := url.Parse(target)
		wm.inbox <- &mention{
			sourceurl,
			targeturl,
		}
		log.Infof("webmention source=%s target=%s enqueued for processing", source, target)
		w.WriteHeader(http.StatusAccepted)
	} else {
		log.Warn("invalid webmention recieved")
		http.Error(w, "Bad Request", http.StatusBadRequest)
	}
}

func (wm *WebMention) processInbox() {
	mention := <-wm.inbox

	res, err := http.Get(mention.source.String())
	if err != nil || res.StatusCode/100 != 2 {
		log.Errorf("Error getting source %s (%s): %s", mention.source, res.Status, err)
		return
	}
	defer res.Body.Close()

	body, err := html.Parse(res.Body)
	if err != nil {
		log.Errorf("Error parsing source %s: %s", mention.source, err)
		return
	}

	found := searchLinks(body, mention.target)
	if found {
		p := microformats.New()
		data := p.ParseNode(body, mention.source)
		if err := wm.Mention(mention.source, mention.target, data); err != nil {
			log.WithError(err).Error("error processing webmention")
		} else {
			log.Infof("processed webmention with mf2 source=%s target=%s", mention.source, mention.target)
		}
		return
	}

	links := GetHeaderLinks(res.Header.Values("Link"))
	if len(links) > 0 {
		if err := wm.Mention(mention.source, mention.target, nil); err != nil {
			log.WithError(err).Error("error processing webmention")
		} else {
			log.Infof("processed webmention without mf2 source=%s target=%s", mention.source, mention.target)
		}
		return
	}

	log.Warnf("no links found on %s", mention.source.String())
}

func (wm *WebMention) processOutbox() {
	mention := <-wm.outbox

	endpoint, err := wm.GetTargetEndpoint(mention.target)
	if err != nil {
		log.WithError(err).Error("error retrieving webmention endpoint")
		return
	}
	if endpoint == nil {
		log.Warn("no webmention endpoint found")
		return
	}
	values := make(url.Values)
	values.Set("source", mention.source.String())
	values.Set("target", mention.target.String())
	if res, err := http.PostForm(endpoint.String(), values); err != nil || (res.StatusCode%100 != 2) {
		log.WithError(err).Errorf(
			"error sending webmention source=%s target=%s status=%s",
			mention.source.String(), mention.target.String(), res.Status,
		)
		return
	}
	log.Infof(
		"successfully sent webmention to %s (source=%s target=%s)",
		endpoint.String(), mention.source.String(), mention.target.String(),
	)
	return
}

func searchLinks(node *html.Node, link *url.URL) bool {
	if node.Type == html.ElementNode {
		if node.DataAtom == atom.A {
			if href := getAttr(node, "href"); href != "" {
				target, err := url.Parse(href)
				if err == nil {
					// prologic/twtxt pods have the form
					// http://pod.domain.tld/external/uri/nick
					if strings.HasPrefix(target.Path, "/external") && target.Query().Get("url") == link.String() {
						return true
					}
					if target.String() == link.String() {
						return true
					}
				}
			}
		}
	}
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		found := searchLinks(c, link)
		if found {
			return found
		}
	}
	return false
}

func getAttr(node *html.Node, name string) string {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, name) {
			return attr.Val
		}
	}
	return ""
}
