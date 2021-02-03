
<a name="0.1.0"></a>
## [0.1.0](https://github.com/prologic/twtxt/compare/0.0.12...0.1.0) (2020-08-19)

### Bug Fixes

* Fix paging on discover and profile views
* Fix missing links in about page
* Fix Dockerfile with missing new pages
* Fix horizontal scroll / overflow on mobile devices
* Fix Atom feed and populate Summary with text/html and title with text/plain
* Fix UX of hashes and shorten them to 11 (by default) characters which is roughly 88 bits of entropy or basically never likely to collide :D
* Fix UX of relative time display and use humanize.Time
* Fix /settings to be a 2-column layout since we don't have that many settings
* Fix superfluous paragraphs in twt formatting
* Fix the email templates to be consistent
* Fix the UX of the password reset view
* Fix formatting of Support Request email and indent/quote Subject/Message
* Fix the workding around password reset emails
* Fix Reply-To for support emails
* Fix email to send text/plain instead of text/html
* Fix wrong template for SendSupportRequestEmail()
* Fix Docker GHA workflow
* Fix docker image versioning
* Fix Docker image
* Fix long option name for open registrations
* Fix bug in /lookup handler
* Fix /lookup to only regturn following and local feeds
* Fix /lookup handler behaviour
* Fix UI/UX of relative twt post time
* Fix UI/UX of date/time of twts
* Fix Content-Type on HEAD /twt/:hash
* Fix a bunch of IE11+ related JS bugs
* Fix Follow/Unfollow actuions on /following view
* Fix feed_cache_last_processing_time_seconds unit
* Fix bug with /lookup handler and perform case insensitive looksup
* Fix and tidy up the /settings view with followers/following now moved to their own views
* Fix missing space on /followers
* Fix user experience with editing your last Twt and preserve the original timestamp
* Fix Atom URL for individual Twts (Fixes [#117](https://github.com/prologic/twtxt/issues/117))
* Fix bad name of PNG (typod extension)
* Fix hash collisions of twts by including the source twtxt URI as well
* Fix and add some missing icons
* Fix bug in new permalink handling
* Fix other missing uploadoptions

### Features

* Add post partial to permalink view for authenticated users so Reply works
* Add WebMentions and basic IndieWeb µFormats v2 support (h-card, h-entry) ([#122](https://github.com/prologic/twtxt/issues/122))
* Add missing spinner icon
* Add tzdata package to runtime docker image
* Add user setting to display dates/times in timezone of choice
* Add Content-Typre to HEAD /twt/:hash handler
* Add HEAD handler for /twt/:hash handler
* Add link to twt.social in footer
* Add feed_cache_last_processing_time_seconds metric
* Add /metrics endpoint for monitoring
* Add external feed ([#118](https://github.com/prologic/twtxt/issues/118))
* Add link to user's profile from settings
* Add Follow/Unfollow actions for the authenticated user on /followers view
* Add /following view with defaults for new to true and tidy up followers view
* Add Twtxt and Atom links to Profile view
* Add a note about off-Github contributions to README
* Add PNG version of twtxt.net logo
* Add support for configurable img whitelist ([#113](https://github.com/prologic/twtxt/issues/113))
* Add permalink support for individual local/external twts ([#112](https://github.com/prologic/twtxt/issues/112))
* Add etags for default avatar ([#111](https://github.com/prologic/twtxt/issues/111))
* Add text/plain alternate rel link to user profiles
* Add docs for Homebrew formulare

### Updates

* Update CHANGELOG for 0.0.13
* Update README.md
* Update README gif ([#121](https://github.com/prologic/twtxt/issues/121))
* Update /feeds view and simplify the actions and remove own feeds from local feeds as they  apprea in my feeds already
* Update the /feeds view with My Feeds and improve some of the wording
* Update README.md ([#116](https://github.com/prologic/twtxt/issues/116))
* Update README.md
* Update logo
* Update README.md


<a name="0.0.12"></a>
## [0.0.12](https://github.com/prologic/twtxt/compare/0.0.11...0.0.12) (2020-08-10)

### Bug Fixes

* Fix duplicate build ids for goreleaser config
* Fix and simplify goreleaser config
* Fix avatar upload handler to resize (disproportionally?) to 60x60
* Fix config file loading for CLI
* Fix install Makefile target
* Fix server Makefile target
* Fix index out of range bug in API for bad clients that don't pass a Token in Headers
* Fix z-index of the top navbar
* Fix logic of count of global followers and following for stats feed bot
* Fix the style of the media upload button and create placeholde rbuttons for other fomratting
* Fix the mediaUpload form entirely by moving it outside the twtForm so it works on IE
* Fix bug pollyfilling the mediaUpload input into the uploadMedia form
* Fix another bug with IE for the uploadMedia capabilities
* Fix script tags inside body
* Fix JS compatibility for Internet Explorer (Fixes [#96](https://github.com/prologic/twtxt/issues/96))
* Fix bad copy/paste in APIv1 spec docs
* Fix error handling for APIv1 /api/v1/follow
* Fix the route for the APIv1 /api/v1/discover endpoint
* Fix error handling of API's isAuthorized() middleware
* Fix updating feed cache on APIv1 /api/v1/post endpoint
* Fix typo in /follow endpoint
* Fix the alignment if the icnos a bit
* Fix bug loading last twt from timeline and discover
* Fix delete last tweet behaviour
* Fix replies on profile views
* Fix techstack document name
* Fix Dockerfile image versioning finally
* Fix wrong handler called for /mentions
* Fix mentions parsing/matching
* Fix binary verisoning
* Fix Dockerfile image and move other sub-packages to the internal namespace too
* Fix typo in profile template

### Documentation

* Document Bitcask's usage in teh Tech Stack

### Features

* Add Homebrew tap to goreleaser config
* Add version string to twtd startup
* Add a basic CLI client with login and post commands ([#108](https://github.com/prologic/twtxt/issues/108))
* Add hashtag search ([#104](https://github.com/prologic/twtxt/issues/104))
* Add FOLLOWERS:%d and FOLLOWING:%d to daily stats feed
* Add section to /help on whot you need to create an account
* Add backend handler /lookup for type-ahead / auot-complete [@mention](https://github.com/mention) lookups from the UI
* Add tooltip for toolbar buttons
* Add &nbsp; between toolbar sections
* Add strikethrough and fixed-width formatting buttons on the toolabr
* Add other formatting uttons
* Add support for syndication formats (RSS, Atom, JSON Feed) ([#95](https://github.com/prologic/twtxt/issues/95))
* Add Pull Request template
* Add Contributor Code of Conduct
* Add Github Downloads README badge
* Add Docker Hub README badges
* Add docs for the APIv1 /api/v1/post and /api/v1/follow endpoints
* Add configuration open to have open user profiles (default: false)
* Add basic e2e integration test framework (just a simple binary)
* Add some more error logging
* Add support for editing and deleting your last Twt ([#88](https://github.com/prologic/twtxt/issues/88))
* Add Contributing section to README
* Add a CNAME (dev.twtxt.net) for developer docs
* Add some basic developer docs
* Add feature to allow users to manage their subFeeds ([#80](https://github.com/prologic/twtxt/issues/80))
* Add basic mentions view and handler
* Add Docker image CI ([#82](https://github.com/prologic/twtxt/issues/82))
* Add MaxUploadSizd to server startup logs
* Add reuseable template partials so we can reuse the post form, posts and pager

### Updates

* Update CHANGELOG for 0.0.12
* Update CHANGELOG for 0.0.12
* Update CHANGELOG for 0.0.12
* Update CHANGELOG for 0.0.12
* Update /about page
* Update issue templates
* Update README.md
* Update APIv1 spec docs, s/Methods/Method/g as all endpoints accept a single-method, if some accept different methods they will be a different endpoint


<a name="0.0.11"></a>
## [0.0.11](https://github.com/prologic/twtxt/compare/0.0.10...0.0.11) (2020-08-02)

### Bug Fixes

* Fix size of external feed icons
* Fix alignment of Twts a bit better (align the actions and Twt post time)
* Fix alignment of uploaded media to be display: block; aligned
* Fix postas functionality post Media Upload (Missing form= attr)
* Fix downscale resolution of media
* Fix bug with appending new media URI to text input
* Fix misuse of pronoun in postas dropdown field
* Fix sourcer links in README
* Fix bad error handling in /settings endpoint for missing avatar_file (Fixes [#63](https://github.com/prologic/twtxt/issues/63))
* Fix potential vulnerability and limit fetches to a configurable limit
* Fix accidental double posting
* Fix /settings handler to limit request body
* Fix followers page ([#53](https://github.com/prologic/twtxt/issues/53))
* Fix wording on settings re showing followers publicly
* Fix bug that incorrectly redirects to the / when you're posting from /discover
* Fix profile template and profile type to show followers correctly with correct link
* Fix Profile.Type setting when calling .Profile() on models
* Fix a few misisng trimSuffix calls in some tempaltes
* Fix session persistence and increase default session timeout to 10days ([#49](https://github.com/prologic/twtxt/issues/49))
* Fix session unmarshalling caused by 150690c
* Fix the mess that is User/Feed URL vs. TwtURL ([#47](https://github.com/prologic/twtxt/issues/47))
* Fix user registration to disallow existing users and feeds
* Fix the specialUsernames feeds for the adminuser properly on twtxt.net
* Fix remainder of feeds on twtxt.net (we lost the contents of news oh well)
* Fix new feed entities on twtxt.net
* Fix all logging in background jobs  to only output warnings
* Fix and tidy up dependencies

### Features

* Add /api/v1/follow endpoint
* Add /api/v1/discover endpoint
* Add /api/v1/timeline endpoint and content negogiation for general NotFound handler
* Add a basic APIv1 set of endpoints ([#66](https://github.com/prologic/twtxt/issues/66))
* Add Media Upload Support ([#69](https://github.com/prologic/twtxt/issues/69))
* Add Etag in AvatarHandler ([#67](https://github.com/prologic/twtxt/issues/67))
* Add meta tags to base template
* Add improved mobile friendly top navbar
* Add logging for SMTP configuration on startup
* Add configuration options for SMTP From addresss used
* Add fixPossibleFeedFollowers migration for twtxt.net
* Add avatar upload to /settings ([#61](https://github.com/prologic/twtxt/issues/61))
* Add update email to /settings (Fixees [#55](https://github.com/prologic/twtxt/issues/55)
* Add Password Reset feature ([#51](https://github.com/prologic/twtxt/issues/51))
* Add list of local (sub)Feeds to the /feeds view for better discovery of user created feeds
* Add Feed model with feed profiles
* Add link to followers
* Add random tweet prompts for a nice variance on the text placeholder
* Add user Avatars to the User Profile view as well
* Add Identicons and support for Profile Avatars ([#43](https://github.com/prologic/twtxt/issues/43))
* Add a flag that allows users to set if the public can see who follows them

### Updates

* Update CHANGELOG for 0.0.11
* Update README.md
* Update README
* Update and improve handling to include conventional (re ...) ([#68](https://github.com/prologic/twtxt/issues/68))
* Update pager wording
* Update pager wording  (It's Twts)
* Update CHANGELOG for 0.0.11
* Update default list of external feeds and add we-are-twtxt
* Update feed sources, refactor and improve the UI/UX by displaying feed sources by source instead of lumped together


<a name="0.0.10"></a>
## [0.0.10](https://github.com/prologic/twtxt/compare/0.0.9...0.0.10) (2020-07-28)

### Bug Fixes

* Fix bug in ExpandMentions
* Fix incorrect log call
* Fix server shutdown and signal handling to listen for SIGTERM and SIGINT
* Fix twtxt.net missing user feeds for prologic (home_datacenter) wtf?!
* Fix missing db.SetUser for fixUserURLs
* Fix another bug in Profile template
* Fix more bugs with User Profile view
* Fix User Profile Latest Tweets
* Fix build and remove unused vars in FixUserAccounts
* Fix User URL and TwtURLs on twtxt.net  and reset them
* Fix Context.IsLocal bug
* Fix bug in User.Is function
* Fix /settings view (again)
* Fix build error (oops silly me)
* Fix bug in /settings vieew
* Fix missing feeds for [@rob](https://github.com/rob) and [@kt84](https://github.com/kt84)  that went missing from their accounts :/
* Fix UI/UX bug in text input with erroneous spaces
* Fix adminUser account on twtxt.net
* Fix user feeds on twtxt.net
* Fix bug with feed creation (case sensitivity)
* Fix Follow/Unfollow local events post v0.9.0 release re URL/TwtURL changes
* Fix numerous bugs post v0.8.0 release (sorry) due to complications  with User Profile URL vs. Feed URL (TwtURL)
* Fix Tweeter.URL on /discover
* Fix syntax error (oops)
* Fix adminUser feeds on twtxt.net
* Fix link to user profiles in user settings followers/following
* Fix Tagline in User Settings so you users can see what they have entered (if it was set)
* Fix User.Following URIs post v0.9.0 break in URIs

### Features

* Add fixAdminUser function to FixUserAccountsJob to add specialUser feeds to the configured AdminUser
* Add SyncStore job to sync data to disk every 1m to prevent accidental data loss
* Add logging when server is shutdown and store is synced/closed
* Add local [@mention](https://github.com/mention) automatic linking for local users and local feeds without a user having to follow  them first

### Updates

* Update CHANGELOG for 0.0.10
* Update README.md
* Update README.md
* Update README.md
* Update startup to merge data store
* Update deps
* Update the FixUserAccounts job and remove all fixes, but leave  the job (we might breka more things)
* Update FixUserAccounts job and remov most of the migration code now that twtxt.net data is fixed
* Update FixUserAccounts job schedule to [@hourly](https://github.com/hourly) and remove adminUser.Feeds hack
* Update  FixUserAccountsJob to eif User URL(s)
* Update FixUserAccounts job back to 1h schedule


<a name="0.0.9"></a>
## [0.0.9](https://github.com/prologic/twtxt/compare/0.0.8...0.0.9) (2020-07-26)

### Features

* Add user profile pages and **BREAKS** existing local user feed URIs ([#27](https://github.com/prologic/twtxt/issues/27))

### Updates

* Update CHANGELOG for 0.0.9


<a name="0.0.8"></a>
## [0.0.8](https://github.com/prologic/twtxt/compare/0.0.7...0.0.8) (2020-07-26)

### Bug Fixes

* Fix the custom release-notes for goreleaser (finally)
* Fix the gorelease custom release notes by skipping the gorelease changelog generation
* Fix the release process, remove git-chglog use before running gorelease
* Fix instructions on how to build from source (Fixes [#30](https://github.com/prologic/twtxt/issues/30))
* Fix PR blocks and remove autoAssign workflow that fails with Resource not accessible by integration
* Fix releasee process to generate release-notes via git-chglog which are better than goreleaser's ones
* Fix goarch in gorelease config (uggh)
* Fix gorelease config (3rd time's the charm)
* Fix gorelease config properly (this time)
* Fix release tools and remove unused shell script
* Fix goreleaser config
* Fix binary release tools and process
* Fix name of twtxt Docker Swarm Stackfile
* Fix docker stack usage in README (Fixes [#34](https://github.com/prologic/twtxt/issues/34))
* Fix typo in feeds template
* Fix error handling for user registrationg and return 404 Feed Not Found for non-existent users/feeds
* Fix build error (oops)
* Fix set of reserved vs. special usernames
* Fix unconstrained no. of user feeds to prevent abuse
* Fix error message when trying to register an account with a previously deleted user (to preserve feeds)
* Fix potential abuse of unconstrained username lengths
* Fix and remove  some useless debugging
* Fix documentation on configuration options and warn about user registration being disabled (Fixes [#29](https://github.com/prologic/twtxt/issues/29))
* Fix the annoying greeting workflow and remove it (it's kind of annoying)

### Features

* Add default configuration env values to docker-compose ([#39](https://github.com/prologic/twtxt/issues/39))
* Add git-chglog to release process to update the ongoing CHANGELOG too
* Add feed creation event to the twtxt special feed
* Add FixUserAccounts job logic to touch feed files for users that have never posted
* Add automated internal special feed
* Add documentation on using docker-compose ([#26](https://github.com/prologic/twtxt/issues/26))
* Add new feature for creating sub-feeds / personas allowing users to create topic-based feeds and poast as those topics
* Add a section to the help page on formatting posts

### Updates

* Update CHANGELOG for 0.0.8
* Update CHANGELOG for 0.0.8
* Update CHANGELOG for 0.0.8
* Update CHANGELOG for 0.0.8
* Update CHANGELOG for 0.0.8
* Update CHANGELOG for 0.0.8
* Update CHANGELOG for 0.0.8


<a name="0.0.7"></a>
## [0.0.7](https://github.com/prologic/twtxt/compare/0.0.6...0.0.7) (2020-07-25)

### Bug Fixes

* Fix .gitignore for ./data/sources
* Fix bug updating followee Followers
* Fix poor spacing between posts on larger devices (Fixes [#28](https://github.com/prologic/twtxt/issues/28))
* Fix and remove accidently commited data/sources file (data is meant to be empty)
* Fix bug with Follow/Unfollow and updating Followers, missed using NormalizeUsername()
* Fix updating Followers on Follow/Unfollow for local users too
* Fix potential nil map bug
* Fix user accounts and populate local Followers
* Fix the settings Followers Follow/Unfollow state
* Fix build system to permit passing VERSION and COMMIT via --build-arg for docker build
* Fix the CI builds to actually build the daemon ([#21](https://github.com/prologic/twtxt/issues/21))

### Features

* Add a convenient UI/UX [Reply] feature on posts
* Add twtxt special feed updates for Follow/Unfollow events from the local instance
* Add confirmation on account deletion in case of accidental clicks
* Add support for faster Docker builds by refactoring the Dockerfile ([#20](https://github.com/prologic/twtxt/issues/20))
* Add Docker Swarmmode Stackfile for production deployments based on https://twtxt.net/ ([#22](https://github.com/prologic/twtxt/issues/22))
* Add local (non-production) docker-compose.yml for reference and Docker-based development ([#25](https://github.com/prologic/twtxt/issues/25))

### Updates

* Update NewFixUserAccountsJob to 1h schedule


<a name="0.0.6"></a>
## [0.0.6](https://github.com/prologic/twtxt/compare/0.0.5...0.0.6) (2020-07-23)

### Bug Fixes

* Fix formatting in FormatRequest
* Fix the session timeout (which was never set0
* Fix some embarassing typos :)
* Fix error handling for parsing feeds and feed sources

### Features

* Add bad feed dtection and log possible bad feeds (no action taken yet)
* Add new feature to detect new followers of feeds on twtxt.net from twtxt User-Agent strings
* Add twtxt to reserve usernames
* Add an improved /about page and add a /help page to help new users

### Updates

* Update README and remove references to the non-existent CLI (this will come later)
* Update default job interval of UpdateFeedSourcesJob


<a name="0.0.5"></a>
## [0.0.5](https://github.com/prologic/twtxt/compare/0.0.4...0.0.5) (2020-07-21)

### Bug Fixes

* Fix UI/UX handling around bad logins
* Fix the follow self feature properly with more consistency
* Fix firefox UI/UX issue by upgrading to PicoCSS v1.0.3 ([#17](https://github.com/prologic/twtxt/issues/17))

### Features

* Add /feeds view with configurable feed sources for discoverability of other sources of feeds
* Add username validation to prevent more potential bad data
* Add configurable theme (site-wide) and persist user-defined (vai cookies) theme selection ([#16](https://github.com/prologic/twtxt/issues/16))
* Add configurable maximum tweet length and cleanup tweets before they are stored to replace new lines, etc


<a name="0.0.4"></a>
## [0.0.4](https://github.com/prologic/twtxt/compare/0.0.3...0.0.4) (2020-07-21)

### Bug Fixes

* Fix links opening in new window with target=_blank
* Fix typo on support page ([#5](https://github.com/prologic/twtxt/issues/5))
* Fix app versioning and add to base template so we can tell which version of twtxt is running
* Fix bug in TwtfileHandler with case sensitivity of nick param

### Features

* Add delete account support
* Add better layout of tweets so they stand out more
* Add support for Markdown formatting ([#10](https://github.com/prologic/twtxt/issues/10))
* Add pagination support ([#9](https://github.com/prologic/twtxt/issues/9))
* Add Follow/Unfollow to /discover view that understands the state of who a user follows or doesn't ([#8](https://github.com/prologic/twtxt/issues/8))

### Updates

* Update README.md
* Update README.md


<a name="0.0.3"></a>
## [0.0.3](https://github.com/prologic/twtxt/compare/0.0.2...0.0.3) (2020-07-19)

### Bug Fixes

* Fix bug with NormalizeURL() incorrectly translating https:// to http://
* Fix deps and cleanup unused ones
* Fix the layout of thee /settings view
* Fix a critical bug whereby users could re-register the same username and override someone else's account :/
* Fix username case sensitivity and normalize by forcing all usersnames to be lowercase and whitespace stripped
* Fix useability issue where some UI/UX would toggle between authenticated and unauthentiated state causing confusion

### Features

* Add support for configuring flags from the environment via the same long option names
* Add options to configure session cookie secret and expiry
* Add Contributing guideline and TODO
* Add additional logic to fix null/blank user account URL(s) to the FixUserAccountJob as well
* Add FixUserAccountsJob to fix existing broken accounts that might already exist
* Add new /discover view for convenience access to the global instance's timeline  with easy to use Follow links for discoverability


<a name="0.0.2"></a>
## [0.0.2](https://github.com/prologic/twtxt/compare/0.0.1...0.0.2) (2020-07-19)

### Bug Fixes

* Fix the  follow self behaviour to actually work
* Fix defaults to use the same  ones in twtxt's options
* Fix  URL() of User objects
* Fix import and hard-code no. of tweets to display

### Features

* Add feature whereby new registered users follow themselves by default
* Add support, stargazers and contributing info to READmE
* Add enhanced server capability for graceful/clean shutdowns
* Add /import feature to import multiple feeds at once ([#1](https://github.com/prologic/twtxt/issues/1))

### Updates

* Update feed update frequency to 5m by default


<a name="0.0.1"></a>
## 0.0.1 (2020-07-18)

### Bug Fixes

* Fix release tool
* Fix screenshots
* Fix broken links and incorrect text that hasn't happened yet
* Fix /login CTA link on /register page
* Fix /register CTA link on /login page
* Fix parsing store uri
* Fix bug ensuring feedsDir exists
* Fix Dockerfile

### Features

* Add theme-switcher and remove erroneous prism.js

### Updates

* Update README.md

