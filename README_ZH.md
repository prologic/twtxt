# twtxt

![GitHub All Releases](https://img.shields.io/github/downloads/jointwt/twtxt/total)
![Docker Cloud Build Status](https://img.shields.io/docker/cloud/build/prologic/twtxt)
![Docker Pulls](https://img.shields.io/docker/pulls/prologic/twtxt)
![Docker Image Size (latest by date)](https://img.shields.io/docker/image-size/prologic/twtxt)

![](https://github.com/jointwt/twtxt/workflows/Coverage/badge.svg)
![](https://github.com/jointwt/twtxt/workflows/Docker/badge.svg)
![](https://github.com/jointwt/twtxt/workflows/Go/badge.svg)
![](https://github.com/jointwt/twtxt/workflows/ReviewDog/badge.svg)

[![Go Report Card](https://goreportcard.com/badge/jointwt/twtxt)](https://goreportcard.com/report/jointwt/twtxt)
[![codebeat badge](https://codebeat.co/badges/15fba8a5-3044-4f40-936f-9e0f5d5d1fd9)](https://codebeat.co/projects/github-com-prologic-twtxt-master)
[![GoDoc](https://godoc.org/github.com/jointwt/twtxt?status.svg)](https://godoc.org/github.com/jointwt/twtxt)
[![GitHub license](https://img.shields.io/github/license/jointwt/twtxt.svg)](https://github.com/jointwt/twtxt)

📕 twtxt是一个类似Twitter™的自托管式分散式微博客平台。没有广告，没有跟踪（针对您的内容和数据）！

![](https://twtxt.net/media/XsLsDHuisnXcL6NuUkYguK.png)

> 从技术上讲，它twtxt是Web应用程序和api形式的多用户[twtxt](https://twtxt.readthedocs.io/en/latest/)客户端。它支持多个用户，还直接托管用户供稿，
> 并以最少的用户配置文件提供熟悉的“社交”体验。
> 
> 它还利用Markdown以及照片，视频甚至音频等多媒体来支持“丰富”文本。

> App Store和Play Store还提供了一组[移动APP](https://jointwt.org/goryon/)。

- https://twtxt.net/

> 注意：[詹姆斯·米尔斯](https://github.com/prologic),，在预算有限的情况下，首先在相当便宜的硬件上运行了这个实例（我希望有很多twtxt实例）。请公平使用它，以便每个人都可以平等使用它！请务必在注册之前阅读/ privacy政策（非常简单）并祝您愉快！🤗

> [赞助](#Sponsor)该项目以支持新功能的开发，改进现有功能并修复错误！
> 或[支持](https://twtxt.net)人员联系以获取有关运行自己的Pod的帮助！
> 或托管您自己的Twtxt Feed，并支持我们的[扩展](https://dev.twtxt.net)程序。

![Demo_1](https://user-images.githubusercontent.com/15314237/90351548-cac74b80-dffd-11ea-8288-b347af548465.gif)

## 移动 App 

![](https://jointwt.org/goryon/images/logo.svg)

Goryon for Twt可在App Store和Play商店中使用。

您的移动设备上安装[Goryon](https://jointwt.org/goryon/)

## 托管 Pods

该项目提供了该平台的完全托管式一键式实例，我们称其为[Twt.social](https://twt.social) pods。

请访问 [Twt.social](https://twt.social) 获取您的 pod !

> 注意：截至2020年8月15日（评论 公告 博客），这是完全免费的，我们邀请任何人与我们联系以获取邀请码，成为最早的几个pod所有者之一！

## 安装

### 预编译二进制包

注意：在解决[问题＃250](https://github.com/jointwt/twtxt/issues/250)之前，请不要使用预构建的二进制文件。请从源代码构建或使用[Docker 镜像](https://hub.docker.com/jointwt)。谢谢你。♂‍♂️


首先，请尝试使用[Releases](https://github.com/jointwt/twtxt/releases)页面上可用的预构建二进制包。

### 使用 Homebrew

我们为 MacOS 用户提供了 [Homebrew](https://brew.sh) 包, 包含命令行客户(`twt`)和服务端(`twtd`)程序 

```console
brew tap jointwt/twtxt
brew install twtxt
```

运行服务端:

```console
twtd
```

运行客户端:

```console
twt
```

### 从源代码构建 

如果您熟悉[Go](https://golang.org)开发, 可以使用这种方法:

1. 克隆仓库 (_重要的_)

```console
git clone https://github.com/jointwt/twtxt.git
```

2. 安装依赖项 (_重要的_)

Linux, macOS:

```console
make deps
```
请注意，为了使媒体上载功能正常工作，您需要安装ffmpeg及其关联的-dev软件包。有关可用性和名称，请咨询您的发行版的软件包存储库。

FreeBSD:

- 安装 `gmake`
- 安装 `pkgconf` 及 `pkg-config`
`gmake deps`

3. 构建二进制包

Linux, macOS:

```console
make
```

FreeBSD:

```console
gmake
```


## 使用

### 命令行客户端

1. 登录您的 [Twt.social](https://twt.social) pod:

```#!console
$ ./twt login
INFO[0000] Using config file: /Users/prologic/.twt.yaml
Username:
```

2. 查看您的时间线 

```#!console
$ ./twt timeline
INFO[0000] Using config file: /Users/prologic/.twt.yaml
> prologic (50 minutes ago)
Hey @rosaelefanten 👋 Nice to see you have a Twtxt feed! Saw your [Tweet](https://twitter.com/koehr_in/status/1326914925348982784?s=20) (_or at least I assume it was yours?_). Never heard of `aria2c` till now! 🤣 TIL

> dilbert (2 hours ago)
Angry Techn Writers ‣ https://dilbert.com/strip/2020-11-14
```

3. 发表推文 (_post_):

```#!console
$ ./twt post
INFO[0000] Using config file: /Users/prologic/.twt.yaml
Testing `twt` the command-line client
INFO[0015] posting twt...
INFO[0016] post successful
```

### 使用Docker镜像

运行compose:

```console
docker-compose up -d
```

然后访问: http://localhost:8000/

### Web App

运行 twtd:

```console
twtd -R
```

__NOTE:__ 默认是禁止用户注册的, 使用 `-R` 标记打开注册选项 

然后访问: http://localhost:8000/

下面是一些命令行客户端的配置项:

```console
$ ./twtd --help
Usage of ./twtd:
  -E, --admin-email string          default admin user email (default "support@twt.social")
  -N, --admin-name string           default admin user name (default "Administrator")
  -A, --admin-user string           default admin user to use (default "admin")
      --api-session-time duration   timeout for api tokens to expire (default 240h0m0s)
      --api-signing-key string      secret to use for signing api tokens (default "PLEASE_CHANGE_ME!!!")
  -u, --base-url string             base url to use (default "http://0.0.0.0:8000")
  -b, --bind string                 [int]:<port> to bind to (default "0.0.0.0:8000")
      --cookie-secret string        cookie secret to use secure sessions (default "PLEASE_CHANGE_ME!!!")
  -d, --data string                 data directory (default "./data")
  -D, --debug                       enable debug logging
      --feed-sources strings        external feed sources for discovery of other feeds (default [https://feeds.twtxt.net/we-are-feeds.txt,https://raw.githubusercontent.com/jointwt/we-are-twtxt/master/we-are-bots.txt,https://raw.githubusercontent.com/jointwt/we-are-twtxt/master/we-are-twtxt.txt])
      --magiclink-secret string     magiclink secret to use for password reset tokens (default "PLEASE_CHANGE_ME!!!")
  -F, --max-fetch-limit int         maximum feed fetch limit in bytes (default 2097152)
  -L, --max-twt-length int          maximum length of posts (default 288)
  -U, --max-upload-size int         maximum upload size of media (default 16777216)
  -n, --name string                 set the pod's name (default "twtxt.net")
  -O, --open-profiles               whether or not to have open user profiles
  -R, --open-registrations          whether or not to have open user registgration
      --session-expiry duration     timeout for sessions to expire (default 240h0m0s)
      --smtp-from string            SMTP From to use for email sending (default "PLEASE_CHANGE_ME!!!")
      --smtp-host string            SMTP Host to use for email sending (default "smtp.gmail.com")
      --smtp-pass string            SMTP Pass to use for email sending (default "PLEASE_CHANGE_ME!!!")
      --smtp-port int               SMTP Port to use for email sending (default 587)
      --smtp-user string            SMTP User to use for email sending (default "PLEASE_CHANGE_ME!!!")
  -s, --store string                store to use (default "bitcask://twtxt.db")
  -t, --theme string                set the default theme (default "dark")
  -T, --twts-per-page int           maximum twts per page to display (default 50)
  -v, --version                     display version information
      --whitelist-domain strings    whitelist of external domains to permit for display of inline images (default [imgur\.com,giphy\.com,reactiongifs\.com,githubusercontent\.com])
pflag: help requested
```

## 配置你的 Pod

至少应设置以下选项:

- `-d /path/to/data`
- `-s bitcask:///path/to/data/twtxt.db` (_默认的_)
- `-R` 开放注册.
- `-O` 公开配置.

其他大多数配置值都应通过环境变量来完成

建议配置 Pod “管理员”账号，可以通过以下环境变量设置:

- `ADMIN_USER=username`
- `ADMIN_EMAIL=email`

为了配置用于密码恢复的电子邮件设置以及/support 和/abuse端点，您应该设置适当的`SMTP_`值

**强烈建议**你设置以下值，以确保您的Pod安全: 

- `API_SIGNING_KEY`
- `COOKIE_SECRET`
- `MAGICLINK_SECRET`

这些值应使用安全的随机数生成器生成，并且长度应为64个字符长度。
您可以使用以下Shell代码片段为您的Pod生成上述值的机密信息

```console
$ cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 64 | head -n 1
```

请**勿发布**或**共享**这些值。确保仅在环境变量中设置

## 生产环境部署

### Docker Swarm

您可以使用`twtxt.yaml` , 基于Docker Stack部署 `twtxt` 到 [Docker Swarm](https://docs.docker.com/engine/swarm/)
集群. 这也取决于并使用[Traefik](https://docs.traefik.io/)入口负载均衡器，因此您还必须在集群中对其进行适当配置和运行。

```console
docker stack deploy -c twtxt.yml
```

## 新闻报导

- 07-12-2020: [Console-30](https://console.substack.com/p/console-30) from the [Console](https://console.substack.com/) weekly newslsetter on open-source proejcts.
- 30-11-2020: [Reddit post on r/golang](https://www.reddit.com/r/golang/comments/k3cmzl/twtxt_is_a_selfhosted_twitterlike_decentralised/)

## 赞助

支持twtxt的持续开发！

**赞助**

- 成为赞助商  [赞助商](https://www.patreon.com/prologic)
- Contribute! See [Issues](https://github.com/jointwt/twtxt/issues)

## 贡献

如果您对这个项目有兴趣, 我们很欢迎您通过以下几种方式做出贡献：

- [提交问题](https://github.com/jointwt/twtxt/issues/new) -- 对于任何错误或想法，新功能或常规问题
-  提交一两个PR, 以改进完善项目!

请阅读 [贡献准则](/CONTRIBUTING.md) 和 [开发文档](https://dev.twtxt.net) 或在 [/docs](/docs) 查看更多内容.

> __请注意:__ 如果您想为[Github](https://github.com)之外的项目做出贡献
> 请与我们联系并告知我们！我们已经将此项目镜像到[Gitea](https://gitea.io/en-us/)构建的私有仓库
> 并且可以通过这种方式完全支持外部协作者（甚至通过电子邮件！）

## 贡献者

感谢所有为该项目做出贡献，进行了实战测试，在自己的项目或产品中使用过它，修复了错误，提高了性能甚至修复了文档中的小错字的人！谢谢您，继续为我们贡献力量！

您可以找到一个[AUTHORS](/AUTHORS)文件，其中保存了该项目的贡献者列表。如果您提供公关，请考虑在其中添加您的名字。还有Github自己的贡献者[统计数据](https://github.com/jointwt/twtxt/graphs/contributors)。

[![](https://sourcerer.io/fame/prologic/jointwt/twtxt/images/0)](https://sourcerer.io/fame/prologic/jointwt/twtxt/links/0)
[![](https://sourcerer.io/fame/prologic/jointwt/twtxt/images/1)](https://sourcerer.io/fame/prologic/jointwt/twtxt/links/1)
[![](https://sourcerer.io/fame/prologic/jointwt/twtxt/images/2)](https://sourcerer.io/fame/prologic/jointwt/twtxt/links/2)
[![](https://sourcerer.io/fame/prologic/jointwt/twtxt/images/3)](https://sourcerer.io/fame/prologic/jointwt/twtxt/links/3)
[![](https://sourcerer.io/fame/prologic/jointwt/twtxt/images/4)](https://sourcerer.io/fame/prologic/jointwt/twtxt/links/4)
[![](https://sourcerer.io/fame/prologic/jointwt/twtxt/images/5)](https://sourcerer.io/fame/prologic/jointwt/twtxt/links/5)
[![](https://sourcerer.io/fame/prologic/jointwt/twtxt/images/6)](https://sourcerer.io/fame/prologic/jointwt/twtxt/links/6)
[![](https://sourcerer.io/fame/prologic/jointwt/twtxt/images/7)](https://sourcerer.io/fame/prologic/jointwt/twtxt/links/7)

## 进展

[![Stargazers over time](https://starcharts.herokuapp.com/jointwt/twtxt.svg)](https://starcharts.herokuapp.com/jointwt/twtxt)

## 相关项目

- [rss2twtxt](https://github.com/prologic/rss2twtxt) -- RSS/Atom to [Twtxt](https://twtxt.readthedocs.org) aggregator.
- [Twt.social](https://twt.social) -- Hosted platform for Twt.social pods like [twtxt.net](https://twtxt.net).
- [Goryon](https://github.com/jointwt/goryon) -- Our Flutter iOS and Android Mobile App.
- [Twt.js](https://github.com/jointwt/twt.js) -- Our JavaScript / NodeJS library for using the API.
- [we-are-twtxt](https://github.com/jointwt/we-are-twtxt) -- A voluntary user contributed registry of users, bots and interesting feeds.
- [jointwt.org](https://github.com/jointwt/jointwt.org) -- Our [JoinTwt.org](https://jointwt.org) landing page.


## 开源协议

`twtxt` 是基于 [MIT 协议](/LICENSE) 构建
