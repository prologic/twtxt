
<a name="0.0.7"></a>
## [0.0.7](https://github.com/prologic/twtxt/compare/0.0.6...0.0.7) (2020-07-24)

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

