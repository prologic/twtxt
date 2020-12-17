---
layout: page
title: "Hash Tag Extension"
category: doc
date: 2020-12-16 17:00:00
order: 3
---

At [twtxt.net](https://twtxt.net/) the **Hash Tag** was invented as an
extension to the original [Twtxt File Format
Specification](https://twtxt.readthedocs.io/en/latest/user/twtxtfile.html#format-specification).

## Purpose

Users might want to tag their twts with labels to make their contents
detectable by other users who are searching for a certain topic. Those labels
are called hash tags. They can be used to group several twts on a specific
topic. Hash tags enable crossreferencing of twts.

## Format

Inspired by twtxt mentions, which use the `@<nick url>` syntax, hash tags are
in the form of `#<tag url>`. Tag and URL are separated by a single space
character. Multiple whitespace characters might be used, but their use is
discouraged.

Both tag and URL are mandatory and must not contain any whitespace. This
extension does not specify a way to escape whitespace in the tag or URL parts.
If the text between `#<` and `>` cannot exactly be split into two parts – tag
and URL – the whole sequence must be treated as regular plain text.

Tags must only contain lower or upper case ASCII letters, numbers, underscores
and minus signs. Other characters are not allowed. A valid tag conforms to the
following regular expression: `[a-zA-Z0-9_-]+`.

The closing angled bracked (`>`) cannot be escaped in the URL, other than using
its URL-escaped form `%3E`.

The URL must point to a resource which lists twts for the given tag.

Clients should render the hash symbol (`#`) right in front of the tag. Hence,
the name "hash tag". Given the hash tag `#<foo https://example.com/tags/foo>`
in a twt a terminal client might show it as:

```
#foo (https://example.com/tags/foo)
```

A web UI might generate HTML like:

```
<a href="https://example.com/tags/foo">#foo</a>
```

## Security Considerations

Clients supporting this extension should provide a way to show the full URL to
the users in advance, so that users are able to see, where they would end up
when following the URL. This way users can abort and decide against visiting a
URL.

Clients may also provide a way to disable hash tag URL folding entirely and
always render the URL next to the hash tag in full length.

