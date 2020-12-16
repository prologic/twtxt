---
layout: page
title: "Twtxt Link Markup Extension"
category: doc
date: 2020-12-16 17:00:00
order: 3
---

At [twtxt.net](https://twtxt.net/) the **Twtxt Link Markup** was invented as an
extension to the original [Twtxt File Format
Specification](https://twtxt.readthedocs.io/en/latest/user/twtxtfile.html#format-specification).

## Purpose

Users might want to link to other resources in their twts. Inserting a plain
URL in the twt works just fine, but sometimes an often much shorter link title
should be provided as well to increase readability. This way clients can
present only the link title to the user while still being able to open the link
URL.

## Format

Inspired by twtxt mentions, which use the `@<nick url>` syntax, twtxt links are
in the form of `#<title url>`. Twtxt link title and twtxt link URL are
separated by a single space character. Multiple whitespace characters might be
used, but their use is discouraged.

-----BEGIN ALTERNATIVE 1-----
Both twtxt link title and twtxt link URL are mandatory and must not contain any
whitespace. This extension does not specify a way to escape whitespace in the
title or URL parts. If the text between `#<` and `>` cannot exactly be split
into two parts – title and URL – the whole sequence must be treated as regular
plain text.
-----END ALTERNATIVE 1-----

-----BEGIN ALTERNATIVE 2-----
Both twtxt link title and twtxt link URL are mandatory, the title may contain
whitespace, however, the URL must not. The last part must be the URL where
everything in front of it is the link title. E.g.

```
#<Hello https://example.com/hello-world>
→ twtxt link title: Hello
  twtxt link URL: https://example.com/hello-world

#<Hello World, foo, bar https://example.com/foo/bar/>
→ twtxt link title: Hello World, foo, bar
  twtxt link URL: https://example.com/foo/bar/
```
-----END ALTERNATIVE 2-----

-----BEGIN ALTERNATIVE 1-----
All optional whitespace around link title and link URL must be stripped.
-----END ALTERNATIVE 1-----

-----BEGIN ALTERNATIVE 2-----
// whitespace around is just not allowed
-----END ALTERNATIVE 2-----


The closing angled bracked (`>`) cannot be escaped, neither in the twtxt link
text nor the twtxt link URL.

-----BEGIN ALTERNATIVE 1-----
However, when used in the twt subject, the URL part of the twtxt link is
optional. The twtxt link title must be the [hash of the referenced
twt](twthashextension.html). If necessary, e.g. when users want to visit the
link, clients are supposed to generate a URL on the fly using the hash from the
link title. A conversation URL for the twt subject may be in the form of
`https://twtxt.net/conf/$HASH`. The exact conversation base URL or template
should be configurable by users in the clients.

The rationale behind this is to save space and bandwidth. Users following the
conversation might not need the extra URL anyways.
-----END ALTERNATIVE 1-----

-----BEGIN ALTERNATIVE 2-----
// no exception for subjects
-----END ALTERNATIVE 2-----

## Security Considerations

Clients supporting this extension should provide a way to show the full URL to
the users in advance, so that users are able see, where they would end up when
following the link. This way users can abort and decide against opening a URL.

Clients may also provide a way to disable twtxt link folding entirely and
always render the URL next to the link title in full length.

