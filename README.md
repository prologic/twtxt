# twtxt

![GitHub All Releases](https://img.shields.io/github/downloads/prologic/twtxt/total)
![Docker Cloud Build Status](https://img.shields.io/docker/cloud/build/prologic/twtxt)
![Docker Pulls](https://img.shields.io/docker/pulls/prologic/twtxt)
![Docker Image Size (latest by date)](https://img.shields.io/docker/image-size/prologic/twtxt)

![](https://github.com/prologic/twtxt/workflows/Coverage/badge.svg)
![](https://github.com/prologic/twtxt/workflows/Docker/badge.svg)
![](https://github.com/prologic/twtxt/workflows/Go/badge.svg)
![](https://github.com/prologic/twtxt/workflows/ReviewDog/badge.svg)

[![Go Report Card](https://goreportcard.com/badge/prologic/twtxt)](https://goreportcard.com/report/prologic/twtxt)
[![codebeat badge](https://codebeat.co/badges/15fba8a5-3044-4f40-936f-9e0f5d5d1fd9)](https://codebeat.co/projects/github-com-prologic-twtxt-master)
[![GoDoc](https://godoc.org/github.com/prologic/twtxt?status.svg)](https://godoc.org/github.com/prologic/twtxt) 
[![GitHub license](https://img.shields.io/github/license/prologic/twtxt.svg)](https://github.com/prologic/twtxt)

📕 twtxt is a Self-Hosted, Twitter™-like Decentralised micro-Blogging platform. No ads, no tracking, your content, your data!

![](https://twtxt.net/media/XsLsDHuisnXcL6NuUkYguK.png)

Technically `twtxt` is a [twtxt](https://twtxt.readthedocs.io/en/latest/) client in the form
of a web application. It supports multiple users and
also hosts user feeds directly.

There is also a publicly (_free_) service online available at:

- https://twtxt.net/

> __NOTE:__ I [James Mills](https://github.com/prologic) run this first (_of which I hope to be many_) `twtxt` instance on pretty cheap hardware on a limited budget. Please use it fairly so everyone can enjoy using it equally! Please be sure to read the [/privacy](https://twtxt.net/privacy) policy before signing up (_pretty striaght forward_) and happy Twt'ing! 🤗

> **[Sponsor](#Sponsor)** this project to support the development of new features
> the upcoming Mobile App and much more! Or contact [Support](https://twtxt.net)
> for help with running your own Twtxt!

![Demo_1](https://user-images.githubusercontent.com/15314237/89133451-30cdb200-d4d9-11ea-8a11-aab67c6fac6a.gif)

## Installation

### Pre-built Binaries

As a first point, please try to use one of the pre-built binaries  that are
available on the [Releases](https://github.com/prologic/twtxt/releases) page.

### Using Homebrew

We provide [Homebrew](https://brew.sh) formulae for macOS users for both the
command-line client (`twt`) as well as the server (`twtd`).

```console
brew tap prologic/twtxt
brew install twtxt
```

Run the server:

```console
twtd
```

Run the commanad-line client:

```console
twt
```

### Building from source

This is an option if you are familiar with [Go](https://golang.org) development.

1. Clone this repository (_this is important_)

```console
git clone https://github.com/prologic/twtxt.git
```

2. Install required dependencies (_this is important_)

```console
make deps
```

3. Build the binaries

```console
make
```

__NOTE___: It is important you follow these steps and don't just simply attempt
           `go get ...` this project as that will not work ([#30](https://github.com/prologic/twtxt/issues/30)) due to the
           need to package templates and static assets which we use the
           [go.rice](https://github.com/GeertJohan/go.rice) tool for.

## Usage

### Deploy with Docker Compose

Run the compose configuration:

```console
docker-compose up -d
```

Then visit: http://localhost:8000/

### Web App

Run twtd:

```console
twtd -r
```

__NOTE:__ Registrations are disabled by default so hence the `-r` flag above.

Then visit: http://localhost:8000/

You can configure other options by specifying them on the command-line:

```console
$ ./twtd -h
Usage of ./twtd:
  -A, --admin-user string           default admin user to use (default "admin")
      --api-session-time duration   Maximum TTL for API tokens (default 240h0m0s)
      --api-signing-key string      API JWT signing key for tokens (default "PLEASE_CHANGE_ME!!!")
  -u, --base-url string             base url to use (default "http://0.0.0.0:8000")
  -b, --bind string                 [int]:<port> to bind to (default "0.0.0.0:8000")
  -S, --cookie-secret string        cookie secret to use (default "PLEASE_CHANGE_ME!!!")
  -d, --data string                 data directory (default "./data")
  -D, --debug                       enable debug logging
  -F, --feed-sources strings        external feed sources (default [https://feeds.twtxt.net/we-are-feeds.txt,https://raw.githubusercontent.com/mdom/we-are-twtxt/master/we-are-bots.txt,https://raw.githubusercontent.com/mdom/we-are-twtxt/master/we-are-twtxt.txt])
      --magiclink-secret string     magiclink secret to use for password reset tokens (default "PLEASE_CHANGE_ME!!!")
      --max-fetch-limit int         Maximum feed fetch limit in bytes (default 2097152)
  -L, --max-twt-length int        maximum length of posts (default 288)
  -U, --max-upload-size int         maximum upload size of media (default 16777216)
  -n, --name string                 set the instance's name (default "twtxt.net")
  -r, --register                    enable user registration
  -E, --session-expiry duration     session expiry to use (default 240h0m0s)
      --smtp-from string            SMTP From address to use for email sending (default "PLEASE_CHANGE_ME!!!")
      --smtp-host string            SMTP Host to use for email sending (default "smtp.gmail.com")
      --smtp-pass string            SMTP Pass to use for email sending (default "PLEASE_CHANGE_ME!!!")
      --smtp-port int               SMTP Port to use for email sending (default 587)
      --smtp-user string            SMTP User to use for email sending (default "PLEASE_CHANGE_ME!!!")
  -s, --store string                store to use (default "bitcask://twtxt.db")
  -t, --theme string                set the default theme (default "dark")
  -T, --twts-per-page int         twts per page to display (default 50)
  -v, --version                     display version information
pflag: help requested
```

## Production Deployments

### Docker Swarm

You can deploy `twtxt` to a [Docker Swarm](https://docs.docker.com/engine/swarm/)
cluster by utilising the provided `twtxt.yaml` Docker Stack. This also depends on
and uses the [Traefik](https://docs.traefik.io/) ingress load balancer so you must
also have that configured and running in your cluster appropriately.

```console
docker stack deploy -c twtxt.yml
```

## Stargazers over time

[![Stargazers over time](https://starcharts.herokuapp.com/prologic/twtxt.svg)](https://starcharts.herokuapp.com/prologic/twtxt)

## Sponsor

Support the ongoing development of twtxt!

**Sponsor**

- Become a [Sponsor](https://www.patreon.com/prologic)
- Contribute! See [Issues](https://github.com/prologic/twtxt/issues)

## Contributing

Interested in contributing to this project? You are welcome! Here are some ways
you can contribute:

- [File an Issue](https://github.com/prologic/twtxt/issues/new) -- For a bug,
  or interesting idea you have for a new feature or just general questions.
- Submit a Pull-Request or two! We welcome all PR(s) that improve the project!

Please see the [Contributing Guidelines](/CONTRIBUTING.md) and checkout the
[Developer Documentation](https://dev.twtxt.net) or over at [/docs](/docs).

> __Please note:__ If you wish to contribute to this proejct off-[Github](https://github.com)
> please get in touch with us and let us know! We have this project mirroed to
> private Git hosting using [Gitea](https://gitea.io/en-us/) and can fully support
> external collaborator this way (_even via email!_).

## Contributors

Thank you to all those that have contributed to this project, battle-tested it, used it in their own projects or products, fixed bugs, improved performance and even fix tiny typos in documentation! Thank you and keep contributing!

You can find an [AUTHORS](/AUTHORS) file where we keep a list of contributors to the project. If you contriibute a PR please consider adding your name there. There is also Github's own [Contributors](https://github.com/prologic/twtxt/graphs/contributors) statistics.

[![](https://sourcerer.io/fame/prologic/prologic/twtxt/images/0)](https://sourcerer.io/fame/prologic/prologic/twtxt/links/0)
[![](https://sourcerer.io/fame/prologic/prologic/twtxt/images/1)](https://sourcerer.io/fame/prologic/prologic/twtxt/links/1)
[![](https://sourcerer.io/fame/prologic/prologic/twtxt/images/2)](https://sourcerer.io/fame/prologic/prologic/twtxt/links/2)
[![](https://sourcerer.io/fame/prologic/prologic/twtxt/images/3)](https://sourcerer.io/fame/prologic/prologic/twtxt/links/3)
[![](https://sourcerer.io/fame/prologic/prologic/twtxt/images/4)](https://sourcerer.io/fame/prologic/prologic/twtxt/links/4)
[![](https://sourcerer.io/fame/prologic/prologic/twtxt/images/5)](https://sourcerer.io/fame/prologic/prologic/twtxt/links/5)
[![](https://sourcerer.io/fame/prologic/prologic/twtxt/images/6)](https://sourcerer.io/fame/prologic/prologic/twtxt/links/6)
[![](https://sourcerer.io/fame/prologic/prologic/twtxt/images/7)](https://sourcerer.io/fame/prologic/prologic/twtxt/links/7)

## Related Projects

- [rss2twtxt](https://github.com/prologic/rss2twtxt)

## License

`twtxt` is licensed under the terms of the [MIT License](/LICENSE)
