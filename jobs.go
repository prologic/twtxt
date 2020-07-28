package twtxt

import (
	"fmt"
	"strings"

	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
)

var Jobs map[string]JobFactory

func init() {
	Jobs = map[string]JobFactory{
		"@every 1m":  NewSyncStoreJob,
		"@every 5m":  NewUpdateFeedsJob,
		"@every 15m": NewUpdateFeedSourcesJob,
		"@hourly":    NewFixUserAccountsJob,
		"@daily":     NewStatsJob,
	}
}

type JobFactory func(conf *Config, store Store) cron.Job

type SyncStoreJob struct {
	conf *Config
	db   Store
}

func NewSyncStoreJob(conf *Config, db Store) cron.Job {
	return &SyncStoreJob{conf: conf, db: db}
}

func (job *SyncStoreJob) Run() {
	if err := job.db.Sync(); err != nil {
		log.WithError(err).Warn("error sycning store")
	}
	log.Info("synced store")
}

type StatsJob struct {
	conf *Config
	db   Store
}

func NewStatsJob(conf *Config, db Store) cron.Job {
	return &StatsJob{conf: conf, db: db}
}

func (job *StatsJob) Run() {
	users, err := job.db.GetAllUsers()
	if err != nil {
		log.WithError(err).Warn("unable to get all users from database")
		return
	}

	log.Infof("updating stats")

	var feeds int
	for _, user := range users {
		feeds += len(user.Feeds)
	}

	tweets, err := GetAllTweets(job.conf)
	if err != nil {
		log.WithError(err).Warnf("error calculating number of tweets")
		return
	}

	text := fmt.Sprintf(
		"🧮  USERS:%d FEEDS:%d POSTS:%d",
		len(users), feeds, len(tweets),
	)

	if err := AppendSpecial(job.conf, job.db, "stats", text); err != nil {
		log.WithError(err).Warn("error updating stats feed")
	}
}

type UpdateFeedsJob struct {
	conf *Config
	db   Store
}

func NewUpdateFeedsJob(conf *Config, db Store) cron.Job {
	return &UpdateFeedsJob{conf: conf, db: db}
}

func (job *UpdateFeedsJob) Run() {
	users, err := job.db.GetAllUsers()
	if err != nil {
		log.WithError(err).Warn("unable to get all users from database")
		return
	}

	log.Infof("updating feeds for %d users", len(users))

	sources := make(map[string]string)

	for _, user := range users {
		for u, n := range user.sources {
			sources[n] = u
		}
	}

	log.Infof("updating %d sources", len(sources))

	cache, err := LoadCache(job.conf.Data)
	if err != nil {
		log.WithError(err).Warn("error loading feed cache")
		return
	}

	cache.FetchTweets(job.conf, sources)

	if err := cache.Store(job.conf.Data); err != nil {
		log.WithError(err).Warn("error saving feed cache")
	} else {
		log.Info("updated feed cache")
	}
}

type UpdateFeedSourcesJob struct {
	conf *Config
	db   Store
}

func NewUpdateFeedSourcesJob(conf *Config, db Store) cron.Job {
	return &UpdateFeedSourcesJob{conf: conf, db: db}
}

func (job *UpdateFeedSourcesJob) Run() {
	log.Infof("updating %d feed sources", len(job.conf.FeedSources))

	feedsources := FetchFeedSources(job.conf.FeedSources)

	log.Infof("fetched %d feed sources", len(feedsources.Sources))

	if err := SaveFeedSources(feedsources, job.conf.Data); err != nil {
		log.WithError(err).Warn("error saving feed sources")
	} else {
		log.Info("updated feed sources")
	}
}

type FixUserAccountsJob struct {
	conf *Config
	db   Store
}

func NewFixUserAccountsJob(conf *Config, db Store) cron.Job {
	return &FixUserAccountsJob{conf: conf, db: db}
}

func (job *FixUserAccountsJob) Run() {
	fixUserURLs := func(user *User) error {
		baseURL := NormalizeURL(strings.TrimSuffix(job.conf.BaseURL, "/"))

		// Reset User URL
		user.URL = URLForUser(baseURL, user.Username)

		for nick, url := range user.Following {
			url = NormalizeURL(url)
			if strings.HasPrefix(url, baseURL) {
				user.Following[nick] = URLForUser(baseURL, nick)
			}
		}

		for nick, url := range user.Followers {
			url = NormalizeURL(url)
			if strings.HasPrefix(url, baseURL) {
				user.Followers[nick] = URLForUser(baseURL, nick)
			}
		}

		if err := job.db.SetUser(user.Username, user); err != nil {
			log.WithError(err).Warnf("error updating user object %s", user.Username)
			return err
		}

		log.Infof("fixed URLs for user %s", user.Username)

		return nil
	}

	fixFeedURLs := func(feed *Feed) error {
		baseURL := NormalizeURL(strings.TrimSuffix(job.conf.BaseURL, "/"))

		// Reset Feed URL
		feed.URL = URLForUser(baseURL, feed.Name)

		for nick, url := range feed.Followers {
			url = NormalizeURL(url)
			if strings.HasPrefix(url, baseURL) {
				feed.Followers[nick] = URLForUser(baseURL, nick)
			}
		}

		if err := job.db.SetFeed(feed.Name, feed); err != nil {
			log.WithError(err).Warnf("error updating feeed object %s", feed.Name)
			return err
		}

		log.Infof("fixed URLs for feed %s", feed.Name)

		return nil
	}

	users, err := job.db.GetAllUsers()
	if err != nil {
		log.WithError(err).Warnf("error loading all user objects")
	} else {
		for _, user := range users {
			if err := fixUserURLs(user); err != nil {
				log.WithError(err).Warnf("error fixing user URLs for %s", user.Username)
			}
		}
	}

	feeds, err := job.db.GetAllFeeds()
	if err != nil {
		log.WithError(err).Warnf("error loading all feed objects")
	} else {
		for _, feed := range feeds {
			if err := fixFeedURLs(feed); err != nil {
				log.WithError(err).Warnf("error fixing feed URLs for %s", feed.Name)
			}
		}
	}

	fixAdminUser := func() error {
		log.Infof("fixing adminUser account %s", job.conf.AdminUser)
		adminUser, err := job.db.GetUser(job.conf.AdminUser)
		if err != nil {
			log.WithError(err).Warnf("error loading user object for AdminUser")
			return err
		}

		for _, feed := range specialUsernames {
			if err := CreateFeed(job.conf, job.db, adminUser, feed, true); err != nil {
				log.WithError(err).Warnf("error creating new feed %s for adminUser", feed)
			}
		}

		if err := job.db.SetUser(adminUser.Username, adminUser); err != nil {
			log.WithError(err).Warn("error saving user object for AdminUser")
			return err
		}

		return nil
	}

	// Fix/Update the adminUser account
	if err := fixAdminUser(); err != nil {
		log.WithError(err).Warnf("error fixing adminUser %s", job.conf.AdminUser)
	}

	// Create twtxtBots feeds
	for _, feed := range twtxtBots {
		if err := CreateFeed(job.conf, job.db, nil, feed, true); err != nil {
			log.WithError(err).Warnf("error creating new feed %s", feed)
		}
	}

}
