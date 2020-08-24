package internal

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

var (
	Jobs        map[string]JobSpec
	StartupJobs map[string]JobSpec
)

func init() {
	Jobs = map[string]JobSpec{
		"SyncStore":         NewJobSpec("@every 1m", NewSyncStoreJob),
		"UpdateFeeds":       NewJobSpec("@every 5m", NewUpdateFeedsJob),
		"SyncCache":         NewJobSpec("@every 10m", NewSyncCacheJob),
		"UpdateFeedSources": NewJobSpec("@every 15m", NewUpdateFeedSourcesJob),
		"FixUserAccounts":   NewJobSpec("@hourly", NewFixUserAccountsJob),
		"DeleteOldSessions": NewJobSpec("@hourly", NewDeleteOldSessionsJob),
		"Stats":             NewJobSpec("@daily", NewStatsJob),
	}

	StartupJobs = map[string]JobSpec{
		"UpdateFeedSources": Jobs["UpdateFeedSources"],
		"FixUserAccounts":   Jobs["FixUserAccounts"],
		"DeleteOldSessions": Jobs["DeleteOldSessions"],
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

	localTwts := job.cache.GetByPrefix(job.conf.BaseURL, false)

	text := fmt.Sprintf(
		"🧮  USERS:%d FEEDS:%d POSTS:%d FOLLOWERS:%d FOLLOWING:%d",
		len(users), len(feeds), len(localTwts), len(followers), len(following),
	)

	if _, err := AppendSpecial(job.conf, job.db, "stats", text); err != nil {
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

	log.Infof("updating feeds for %d users and  %d feeds", len(users), len(feeds))

	sources := make(map[string]string)

	// Ensure all specialUsername feeds are in the cache
	for _, username := range specialUsernames {
		sources[username] = URLForUser(job.conf, username)
	}

	// Ensure all twtxtBots feeds are in the cache
	for _, bot := range twtxtBots {
		sources[bot] = URLForUser(job.conf, bot)
	}

	for _, feed := range feeds {
		// Ensure we fetch the feed's own posts in the cache
		sources[feed.Name] = feed.URL
	}

	for _, user := range users {
		// Ensure we fetch the user's own posts in the cache
		sources[user.Username] = user.URL
		for u, n := range user.sources {
			sources[n] = u
		}
	}

	log.Infof("updating %d sources", len(sources))
	job.cache.FetchTwts(job.conf, sources)

	log.Infof("warming cache with local twts for %s", job.conf.BaseURL)
	job.cache.GetByPrefix(job.conf.BaseURL, true)

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

type DeleteOldSessionsJob struct {
	conf  *Config
	cache Cache
	db    Store
}

func NewDeleteOldSessionsJob(conf *Config, cache Cache, db Store) cron.Job {
	return &DeleteOldSessionsJob{conf: conf, cache: cache, db: db}
}

func (job *DeleteOldSessionsJob) Run() {
	log.Infof("deleting old sessions")

	sessions, err := job.db.GetAllSessions()
	if err != nil {
		log.WithError(err).Error("error loading seessions")
		return
	}

	for _, session := range sessions {
		if session.Expired() {
			log.Infof("deleting expired session %s", session.ID)
			if err := job.db.DelSession(session.ID); err != nil {
				log.WithError(err).Error("error deleting session object")
			}
		}
	}
}
