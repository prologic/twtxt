---
layout: page
title: "Twt Subject Extension"
category: doc
date: 2020-12-22 15:00:00
order: 3
---

At [twtxt.net](https://twtxt.net/) the **Twt Subject** was invented as an
extension to the original [Twtxt File Format
Specification](https://twtxt.readthedocs.io/en/latest/user/twtxtfile.html#format-specification).

## Purpose

Twts in their purest form provide only the mentions mechanism to reply to
certain twtxt users. This works well in small, low traffic twtxt communities.
However, if there are several ongoing discussions at the same time, a single
mention may not be enough for consuming twxt users to clearly identify the
exact conversation this twt is considered part of by its author. So twtxt users
quickly started to provide more context in parentheses at the beginning of the
twt right after any mentions – the so called subject – in the form of:

```
@<nick1 url1> @<nick2 url2> (re: topic) That's what I think as well.
                            ^^^^^^^^^^^
                            traditional subject
```

The twt subject provides a mechanism to specify references in twt replies and
thus group twts into entire conversations.

## Format

The twt subject is the very first contents in parentheses right after any
optional mentions in the twt text. The opening and closing parentheses are part
of the subject. Apart from mentions and whitespace, there must not be any other
text preceding the subject or else the parenthesized text must be treated as
regular text.

To further improve traditional subjects with only references in natural
language, the [Twt Hash](twthashextension.html) of the first twt starting the
conversation must be used in form of a [Hash Tag](hashtagextension.html) in the
twt subject. This allows clients to easily group several twts to conversations.

The hash tag may be surrounded with other text, although this is discouraged.
There must be exactly one hash tag in the subject. The synatx is:

```
(#<hash url>) text
^^^^^^^^^^^^^
twt subject
```

Or:

```
@<nick url> (#<hash url>) text
            ^^^^^^^^^^^^^
            twt subject
```

Clients must only use the tag (the twt hash) part of the hash tag rather than
its URL when grouping twts to allow for different twtxt.net instance URLs being
involved. This way users are able to point the twt hash tag URL to their own
instances without running the risk of clients splitting up conversations. It's
also possible to reference different endpoints, such as single twt, tag search
or conversation view.

A conversation uses a single twt hash in all the subjects throughout the whole
discussion, namely the one from the twt starting the conversation. In order to
branch off, the appropriate twt hash can be used in the following subjects to
form complex conversation trees rather than just linear flows.

Clients may hide the subjects to use the available space more efficiently for
contents.

Examples of replies referencing a twt with hash "abcdefg" (keep in mind these
twts are on one physical line, but may be rendered in several ones depending on
your font size and screen width):

```
@<joe https://example.com/twtxt.txt> @<kate https://example.org/twtxt.txt> (#<abcdefg https://example.com/search?tag=abcdefg>) Yes, I agree.
```

```
(#<abcdefg https://example.org/conv/abcdefg>) But what about…?
```

```
@<joe https://example.com/twtxt.txt> (#<abcdefg https://example.com/search?tag=abcdefg>) @<kate https://example.org/twtxt.txt> Hmm, are you sure? What if…
```
