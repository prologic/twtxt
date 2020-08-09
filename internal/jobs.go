package twtxt

import (
	"fmt"

	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
)

// JobSpec ...
type JobSpec struct {
	Schedule string
	Factory  JobFactory
}

func NewJobSpec(schedule string, factory JobFactory) JobSpec {
	return JobSpec{schedule, factory}
}

var Jobs map[string]JobSpec

func init() {
	Jobs = map[string]JobSpec{
		"SyncStore":         NewJobSpec("@every 1m", NewSyncStoreJob),
		"UpdateFeeds":       NewJobSpec("@every 5m", NewUpdateFeedsJob),
		"SyncCache":         NewJobSpec("@every 10m", NewSyncCacheJob),
		"UpdateFeedSources": NewJobSpec("@every 15m", NewUpdateFeedSourcesJob),
		"FixUserAccounts":   NewJobSpec("@hourly", NewFixUserAccountsJob),
		"Stats":             NewJobSpec("@daily", NewStatsJob),
	}
}

type JobFactory func(conf *Config, cache Cache, store Store) cron.Job

type SyncStoreJob struct {
	conf  *Config
	cache Cache
	db    Store
}

func NewSyncStoreJob(conf *Config, cache Cache, db Store) cron.Job {
	return &SyncStoreJob{conf: conf, cache: cache, db: db}
}

func (job *SyncStoreJob) Run() {
	if err := job.db.Sync(); err != nil {
		log.WithError(err).Warn("error sycning store")
	}
	log.Info("synced store")
}

type StatsJob struct {
	conf  *Config
	cache Cache
	db    Store
}

func NewStatsJob(conf *Config, cache Cache, db Store) cron.Job {
	return &StatsJob{conf: conf, cache: cache, db: db}
}

func (job *StatsJob) Run() {
	var (
		followers []string
		following []string
	)

	log.Infof("updating stats")

	feeds, err := job.db.GetAllFeeds()
	if err != nil {
		log.WithError(err).Warn("unable to get all feeds from database")
		return
	}

	users, err := job.db.GetAllUsers()
	if err != nil {
		log.WithError(err).Warn("unable to get all users from database")
		return
	}

	for _, feed := range feeds {
		followers = append(followers, MapStrings(StringValues(feed.Followers), NormalizeURL)...)
	}

	for _, user := range users {
		followers = append(followers, MapStrings(StringValues(user.Followers), NormalizeURL)...)
		following = append(following, MapStrings(StringValues(user.Following), NormalizeURL)...)
	}

	followers = UniqStrings(followers)
	following = UniqStrings(following)

	twts, err := GetAllTwts(job.conf)
	if err != nil {
		log.WithError(err).Warnf("error calculating number of twts")
		return
	}

	text := fmt.Sprintf(
		"🧮  USERS:%d FEEDS:%d POSTS:%d FOLLOWERS:%d FOLLOWING:%d",
		len(users), len(feeds), len(twts), len(followers), len(following),
	)

	if err := AppendSpecial(job.conf, job.db, "stats", text); err != nil {
		log.WithError(err).Warn("error updating stats feed")
	}
}

type UpdateFeedsJob struct {
	conf  *Config
	cache Cache
	db    Store
}

func NewUpdateFeedsJob(conf *Config, cache Cache, db Store) cron.Job {
	return &UpdateFeedsJob{conf: conf, cache: cache, db: db}
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

	job.cache.FetchTwts(job.conf, sources)

	log.Info("updated feed cache")
}

type SyncCacheJob struct {
	conf  *Config
	cache Cache
	db    Store
}

func NewSyncCacheJob(conf *Config, cache Cache, db Store) cron.Job {
	return &SyncCacheJob{conf: conf, cache: cache, db: db}
}

func (job *SyncCacheJob) Run() {
	if err := job.cache.Store(job.conf.Data); err != nil {
		log.WithError(err).Warn("error saving feed cache")
		return
	}

	log.Info("synced feed cache")
}

type UpdateFeedSourcesJob struct {
	conf  *Config
	cache Cache
	db    Store
}

func NewUpdateFeedSourcesJob(conf *Config, cache Cache, db Store) cron.Job {
	return &UpdateFeedSourcesJob{conf: conf, cache: cache, db: db}
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
	conf  *Config
	cache Cache
	db    Store
}

func NewFixUserAccountsJob(conf *Config, cache Cache, db Store) cron.Job {
	return &FixUserAccountsJob{conf: conf, cache: cache, db: db}
}

func (job *FixUserAccountsJob) Run() {
	// TODO: Refactor this into its own job.
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
