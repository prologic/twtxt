package internal

import (
	"fmt"

	"github.com/jointwt/twtxt/types"
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
		"UpdateFeedSources": NewJobSpec("@every 15m", NewUpdateFeedSourcesJob),

		"DeleteOldSessions": NewJobSpec("@hourly", NewDeleteOldSessionsJob),

		"Stats": NewJobSpec("@daily", NewStatsJob),

		"CreateBots":       NewJobSpec("", NewCreateBotsJob),
		"CreateAdminFeeds": NewJobSpec("", NewCreateAdminFeedsJob),
	}

	StartupJobs = map[string]JobSpec{
		"UpdateFeeds":       Jobs["UpdateFeeds"],
		"UpdateFeedSources": Jobs["UpdateFeedSources"],
		"CreateBots":        Jobs["CreateBots"],
		"CreateAdminFeeds":  Jobs["CreateAdminFeeds"],
		"DeleteOldSessions": Jobs["DeleteOldSessions"],
	}
}

type JobFactory func(conf *Config, blogs *BlogsCache, cache *Cache, archive Archiver, store Store) cron.Job

type SyncStoreJob struct {
	conf    *Config
	blogs   *BlogsCache
	cache   *Cache
	archive Archiver
	db      Store
}

func NewSyncStoreJob(conf *Config, blogs *BlogsCache, cache *Cache, archive Archiver, db Store) cron.Job {
	return &SyncStoreJob{conf: conf, blogs: blogs, cache: cache, archive: archive, db: db}
}

func (job *SyncStoreJob) Run() {
	if err := job.db.Sync(); err != nil {
		log.WithError(err).Warn("error sycning store")
	}
	log.Info("synced store")
}

type StatsJob struct {
	conf    *Config
	blogs   *BlogsCache
	cache   *Cache
	archive Archiver
	db      Store
}

func NewStatsJob(conf *Config, blogs *BlogsCache, cache *Cache, archive Archiver, db Store) cron.Job {
	return &StatsJob{conf: conf, blogs: blogs, cache: cache, archive: archive, db: db}
}

func (job *StatsJob) Run() {
	var (
		followers []string
		following []string
	)

	log.Infof("updating stats")

	archiveSize, err := job.archive.Count()
	if err != nil {
		log.WithError(err).Warn("unable to get archive size")
		return
	}

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

	var twts int

	allFeeds, err := GetAllFeeds(job.conf)
	if err != nil {
		log.WithError(err).Warn("unable to get all local feeds")
		return
	}
	for _, feed := range allFeeds {
		count, err := GetFeedCount(job.conf, feed)
		if err != nil {
			log.WithError(err).Warnf("error getting feed count for %s", feed)
			return
		}
		twts += count
	}

	text := fmt.Sprintf(
		"🧮 USERS:%d FEEDS:%d TWTS:%d BLOGS:%d ARCHIVED:%d CACHE:%d FOLLOWERS:%d FOLLOWING:%d",
		len(users), len(feeds), twts, job.blogs.Count(),
		archiveSize, job.cache.Count(), len(followers), len(following),
	)

	if _, err := AppendSpecial(job.conf, job.db, "stats", text); err != nil {
		log.WithError(err).Warn("error updating stats feed")
	}
}

type UpdateFeedsJob struct {
	conf    *Config
	blogs   *BlogsCache
	cache   *Cache
	archive Archiver
	db      Store
}

func NewUpdateFeedsJob(conf *Config, blogs *BlogsCache, cache *Cache, archive Archiver, db Store) cron.Job {
	return &UpdateFeedsJob{conf: conf, blogs: blogs, cache: cache, archive: archive, db: db}
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

	sources := make(types.Feeds)
	publicFollowers := make(map[types.Feed][]string)

	// Ensure all specialUsername feeds are in the cache
	for _, username := range specialUsernames {
		sources[types.Feed{Nick: username, URL: URLForUser(job.conf.BaseURL, username)}] = true
	}

	// Ensure all twtxtBots feeds are in the cache
	for _, bot := range twtxtBots {
		sources[types.Feed{Nick: bot, URL: URLForUser(job.conf.BaseURL, bot)}] = true
	}

	for _, feed := range feeds {
		// Ensure we fetch the feed's own posts in the cache
		sources[types.Feed{Nick: feed.Name, URL: feed.URL}] = true
	}

	for _, user := range users {
		for feed := range user.Sources() {
			sources[feed] = true
			if user.IsFollowingPubliclyVisible {
				publicFollowers[feed] = append(publicFollowers[feed], user.Username)
			}
		}
	}

	log.Infof("updating %d sources", len(sources))
	job.cache.FetchTwts(job.conf, job.archive, sources, publicFollowers)

	log.Infof("warming cache with local twts for %s", job.conf.BaseURL)
	job.cache.GetByPrefix(job.conf.BaseURL, true)

	log.Info("updated feed cache")

	log.Info("syncing feed cache ", len(job.cache.Twts))

	if err := job.cache.Store(job.conf.Data); err != nil {
		log.WithError(err).Warn("error saving feed cache")
		return
	}

	log.Info("synced feed cache")

}

type UpdateFeedSourcesJob struct {
	conf    *Config
	blogs   *BlogsCache
	cache   *Cache
	archive Archiver
	db      Store
}

func NewUpdateFeedSourcesJob(conf *Config, blogs *BlogsCache, cache *Cache, archive Archiver, db Store) cron.Job {
	return &UpdateFeedSourcesJob{conf: conf, blogs: blogs, cache: cache, archive: archive, db: db}
}

func (job *UpdateFeedSourcesJob) Run() {
	log.Infof("updating %d feed sources", len(job.conf.FeedSources))

	feedsources := FetchFeedSources(job.conf, job.conf.FeedSources)

	log.Infof("fetched %d feed sources", len(feedsources.Sources))

	if err := SaveFeedSources(feedsources, job.conf.Data); err != nil {
		log.WithError(err).Warn("error saving feed sources")
	} else {
		log.Info("updated feed sources")
	}
}

type CreateAdminFeedsJob struct {
	conf    *Config
	blogs   *BlogsCache
	cache   *Cache
	archive Archiver
	db      Store
}

func NewCreateAdminFeedsJob(conf *Config, blogs *BlogsCache, cache *Cache, archive Archiver, db Store) cron.Job {
	return &CreateAdminFeedsJob{conf: conf, blogs: blogs, cache: cache, archive: archive, db: db}
}

func (job *CreateAdminFeedsJob) Run() {
	log.Infof("creating feeds for adminUser: %s", job.conf.AdminUser)

	if !job.db.HasUser(job.conf.AdminUser) {
		log.Warnf("no adminUser account matching %s", job.conf.AdminUser)
		return
	}

	adminUser, err := job.db.GetUser(job.conf.AdminUser)
	if err != nil {
		log.WithError(err).Warnf("error loading user object for AdminUser")
		return
	}

	for _, feed := range specialUsernames {
		if !job.db.HasFeed(feed) {
			if err := CreateFeed(job.conf, job.db, adminUser, feed, true); err != nil {
				log.WithError(err).Warnf("error creating new feed %s for adminUser", feed)
			}
		}
	}

	if err := job.db.SetUser(adminUser.Username, adminUser); err != nil {
		log.WithError(err).Warn("error saving user object for AdminUser")
	}

}

type CreateBotsJob struct {
	conf    *Config
	blogs   *BlogsCache
	cache   *Cache
	archive Archiver
	db      Store
}

func NewCreateBotsJob(conf *Config, blogs *BlogsCache, cache *Cache, archive Archiver, db Store) cron.Job {
	return &CreateBotsJob{conf: conf, blogs: blogs, cache: cache, archive: archive, db: db}
}

func (job *CreateBotsJob) Run() {
	log.Infof("creating bots ...")

	// Create twtxtBots feeds
	for _, feed := range twtxtBots {
		if !job.db.HasFeed(feed) {
			if err := CreateFeed(job.conf, job.db, nil, feed, true); err != nil {
				log.WithError(err).Warnf("error creating new feed %s", feed)
			}
		}
	}
}

type DeleteOldSessionsJob struct {
	conf    *Config
	blogs   *BlogsCache
	cache   *Cache
	archive Archiver
	db      Store
}

func NewDeleteOldSessionsJob(conf *Config, blogs *BlogsCache, cache *Cache, archive Archiver, db Store) cron.Job {
	return &DeleteOldSessionsJob{conf: conf, blogs: blogs, cache: cache, archive: archive, db: db}
}

func (job *DeleteOldSessionsJob) Run() {
	log.Info("deleting old sessions")

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
