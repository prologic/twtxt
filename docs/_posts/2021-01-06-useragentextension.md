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

Depending on the number of followers on a multi user instance there are two
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

### Multiple Followers

Starting with a second follower, the format changes. It aims to be fairly
compact:

```
<client.name>/<client.version> (~<who-follows.url>; contact=<client.contact>)
```

For example:

```
twtxt/0.1.0@abcdefg (~https://example.com/whoFollows?token=randomtoken123; contact=https://example.com/support)
```

The feed URL and nick from the Single Follower format are replaced with just a
single Who Follows Resource URL, where all followers can be obtained. To aid
parsing and quickly differentiate these `User-Agent` headers from other
software, such as search engine spiders, the Who Follows URL is prefixed with a
tilde (`~`) rather than the plus sign (`+`).

An optional contact URL or e-mail address may be included as well. If present,
this should be either the client operator's e-mail address or a URL pointing to
a page were the client owner can be contacted.

### Who Follows Resource

When requested with the `Accept: application/json` header, this resource must
provide a JSON object with nicks as keys and their *twtxt.txt* file URLs as
values. The Format of the HTTP response body is:

```
{ "<nick>": "<url>" }
```

For example:

```
{
  "somebody": "https://example.com/user/somebody/twtxt.txt",
  "someoneelse": "https://example.com/user/someonelse/twtxt.txt"
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
users, who see such a Who Follows Resource in their web server access logs, to
just swap out the own feed URL in a query parameter for a different feed and
get all the followers of that feed. The easiest way is to use a reasonably long
random token which internally is mapped to the feed URL and only valid for a
short period of time, e.g. one hour. The token should be rotated regularly.

