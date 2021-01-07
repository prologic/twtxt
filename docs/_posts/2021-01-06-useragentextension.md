---
layout: page
title: "Multi User User-Agent Extension"
category: doc
date: 2021-01-06 15:00:00
order: 3
---

At [twtxt.net](https://twtxt.net/) the **Multi User User-Agent** was invented
as an extension to the original [Twtxt Discoverability
Specification](https://twtxt.readthedocs.io/en/latest/user/discoverability.html).

## Purpose

Users can discover their followers if the followers include a specially
formatted `User-Agent` HTTP request header when fetching *twtxt.txt* files. The
original twtxt specification covers only single user clients. Since twtxt.net
is a multi user client, a single `GET` request is enough to present several
users the same feed. However, the `User-Agent` header needs to be modified when
several users on the same client instance are following a certain feed, so that
feed owners are still able to find out about their followers.

## Format

Depending on the number of followers on a multi user instance there are three
different formats to be used in the `User-Agent` HTTP request header.

### Single Follower

If there's only a single follower, the original twtxt specification on
[Discoverability](https://twtxt.readthedocs.io/en/latest/user/discoverability.html)
should be followed, to be backwards-compatible:

```
<client.name>/<client.version> (+<source.url>; @<source.nick>)
```

For example:

```
twtxt/1.2.3 (+https://example.com/twtxt.txt; @somebody)
```

### Two To Five Followers

Starting with a second follower, the format changes. It aims to be fairly
compact:

```
<client.name>/<client.version> (Pod: <hostname> Followers: <nick>… Support: <url>)
```

For example:

```
twtxt/0.1.0@cdd6014 (Pod: example.com Followers: somebody someoneelse Support: https://example.com/support)
```

This information is enough to figure out the exact *twxt.txt* files for all the
followers. Join the `<hostname>` with each of the `<nick>`s using optional
client-specific pre-, in- and/or suffixes. In case of the twtxt.net software
the prefix is `https://`, infix is `/user/` and the suffix `/twtxt.txt`,
resulting in the two feed URLs:

* *https://example.com/user/somebody/twtxt.txt*
* *https://example.com/user/someonelese/twtxt.txt*

Pre-, in- and suffix should be easily discoverable when visiting the client.
Nicks should be in alphabetical order.

The support information is optional and should point to a page were the client
owner can be contacted.

### Six Or More Followers

To avoid `User-Agent` headers getting too large, there should be a limit on the
number of nicks to be included. The exact number when to switch formats is up to
the client author or operator. When six or more users follow the same feed, the
twtxt.net software sends the header in the following format:

```
<client.name>/<client.version> (Pod: <hostname> Followers: <nick1> <nick2> <nick3> <nick4> <nick5> and <number> more... <url> Support: <url>)
```

The `<number>` specifies the amount of users, which are excluded from the nick
list, but which can be obtained from the given URL.

For example:

```
twtxt/0.1.0@cdd6014 (Pod: example.com Followers: somebody someoneelse user3 user4 user5 and 3 more... https://example.com/whoFollows?uri=https://example.com/twtxt.txt&nick=joe&token=R9nWDD23u Support: https://example.com/support)
```

### Who Follows Resource

When requested with the `Accept: application/json` header, this resource must
provide a JSON object with nicks as keys and their *twtxt.txt* file URLs as
values. The mapping must contain all followers, including those who are already
present in the `User-Agent` header. The Format of the HTTP response body is:

```
{ "<nick>": "<url>" }
```

For example:

```
{
  "somebody": "https://example.com/user/somebody/twtxt.txt",
  "someoneelse": "https://example.com/user/someonelse/twtxt.txt",
  "user3": "https://example.com/user/user3/twtxt.txt",
  "user4": "https://example.com/user/user4/twtxt.txt",
  "user5": "https://example.com/user/user5/twtxt.txt",
  "user6": "https://example.com/user/user6/twtxt.txt",
  "user7": "https://example.com/user/user7/twtxt.txt",
  "user8": "https://example.com/user/user8/twtxt.txt"
}
```

## Security Considerations

Users of multi user clients should have the option to keep their following list
secret and thus to hide themselves from both the `User-Agent` as well as Who
Follows Resource.

The Who Follows Resource could be easily guessable and thus must be somehow
protected to not publicly disclose the followers of a certain feed to
unauthorized third parties. Keep in mind, the `User-Agent` header is only
available to the feed owner or web server operator. It must not be possible for
users, who see such a Who Follows Resources in their web server access logs, to
just swap out the own feed URL for a different feed and get all the followers
of that feed. The easiest way is to use a reasonably long random token which
internally is mapped to the feed URL and only valid for a short period of time,
e.g. one hour. The token should be rotated regularly.

