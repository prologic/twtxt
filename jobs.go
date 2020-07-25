package twtxt

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
)

var Jobs map[string]JobFactory

func init() {
	Jobs = map[string]JobFactory{
		"@every 5m":  NewUpdateFeedsJob,
		"@every 15m": NewUpdateFeedSourcesJob,
		"@hourly":    NewFixUserAccountsJob,
		"@daily":     NewStatsJob,
	}
}

type JobFactory func(conf *Config, store Store) cron.Job

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

	if err := AppendSpecial(job.conf.Data, "stats", text); err != nil {
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

	cache.FetchTweets(sources)

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

	feeds := FetchFeeds(job.conf.FeedSources)

	log.Infof("fetched %d feeds", len(feeds))

	if err := SaveFeeds(feeds, job.conf.Data); err != nil {
		log.WithError(err).Warn("error saving feeds")
	} else {
		log.Info("updated feeds")
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
	users, err := job.db.GetAllUsers()
	if err != nil {
		log.WithError(err).Warn("unable to get all users from database")
		return
	}

	// followee -> list of followers
	followers := make(map[string][]string)

	for _, user := range users {
		fn := filepath.Join(filepath.Join(job.conf.Data, feedsDir, user.Username))
		if _, err := os.Stat(fn); os.IsNotExist(err) {
			if err := ioutil.WriteFile(fn, []byte{}, 0644); err != nil {
				log.WithError(err).Warnf("error touching feed file for user %s", user.Username)
			} else {
				log.Infof("touched feed file for user %s", user.Username)
			}
		}

		normalizedUsername := NormalizeUsername(user.Username)

		if normalizedUsername != user.Username {
			log.Infof("migrating user account %s -> %s", user.Username, normalizedUsername)

			if err := job.db.DelUser(user.Username); err != nil {
				log.WithError(err).Errorf("error deleting old user %s", user.Username)
				return
			}

			p := filepath.Join(filepath.Join(job.conf.Data, feedsDir))

			if err := os.Rename(filepath.Join(p, user.Username), filepath.Join(p, fmt.Sprintf("%s.tmp", user.Username))); err != nil {
				log.WithError(err).Errorf("error renaming old feed for %s", user.Username)
				return
			}

			if err := os.Rename(filepath.Join(p, fmt.Sprintf("%s.tmp", user.Username)), filepath.Join(p, normalizedUsername)); err != nil {
				log.WithError(err).Errorf("error renaming new feed for %s", user.Username)
				return
			}

			// Fix Username
			user.Username = normalizedUsername

			// Fix URL
			user.URL = fmt.Sprintf(
				"%s/u/%s",
				strings.TrimSuffix(job.conf.BaseURL, "/"),
				normalizedUsername,
			)

			if err := job.db.SetUser(normalizedUsername, user); err != nil {
				log.WithError(err).Errorf("error migrating user %s", normalizedUsername)
				return
			}

			log.Infof("successfully migrated user account %s", normalizedUsername)
		}

		if user.URL == "" {
			log.Infof("fixing URL for user %s", user.Username)
			// Fix URL
			user.URL = fmt.Sprintf(
				"%s/u/%s",
				strings.TrimSuffix(job.conf.BaseURL, "/"),
				normalizedUsername,
			)

			if err := job.db.SetUser(normalizedUsername, user); err != nil {
				log.WithError(err).Errorf("error migrating user %s", normalizedUsername)
				return
			}

			log.Infof("successfully fixed URL for user %s", user.Username)
		}

		for _, url := range user.Following {
			url = NormalizeURL(url)
			if strings.HasPrefix(url, job.conf.BaseURL) {
				followee := NormalizeUsername(NormalizeUsername(filepath.Base(url)))
				followers[followee] = append(followers[followee], user.Username)
			}
		}
	}

	for followee, followers := range followers {
		user, err := job.db.GetUser(followee)
		if err != nil {
			log.WithError(err).Warnf("error loading user object for %s", followee)
			continue
		}

		if user.Followers == nil {
			user.Followers = make(map[string]string)
		}
		for _, follower := range followers {
			user.Followers[follower] = URLForUser(job.conf.BaseURL, follower)
		}

		if err := job.db.SetUser(followee, user); err != nil {
			log.WithError(err).Warnf("error saving user object for %s", followee)
			continue
		}
		log.Infof("updating %d followers for %s", len(followers), followee)
	}

	adminUser, err := job.db.GetUser(job.conf.AdminUser)
	if err != nil {
		log.WithError(err).Warnf("error loading user object for AdminUser")
	} else {
		for _, specialUser := range specialUsernames {
			if !adminUser.OwnsFeed(specialUser) {
				adminUser.Feeds = append(adminUser.Feeds, specialUser)
			}
		}
		if err := job.db.SetUser(adminUser.Username, adminUser); err != nil {
			log.WithError(err).Warn("error saving user object for AdminUser")
		} else {
			log.Infof("updated AdminUser %s with %d specialUsername feeds", job.conf.AdminUser, len(specialUsernames))
		}
	}
}
