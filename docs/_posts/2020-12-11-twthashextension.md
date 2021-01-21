---
layout: page
title: "Twt Hash Extension"
category: doc
date: 2020-12-11 15:00:00
order: 3
---

At [twtxt.net](https://twtxt.net/) the **Twt Hash** was invented as an
extension to the original [Twtxt File Format
Specification](https://twtxt.readthedocs.io/en/latest/user/twtxtfile.html#format-specification).

## Purpose

Twt hashes make twts identifiable, so replies can be created to build up
conversations. The twt's hash is used in the [Twt
Subject](twtsubjectextension.html) of the reply twt to indicate to which
original twt it refers to. The twt hash is similar to the `Message-ID` header
of an e-mail which the response e-mail would reference in its `In-Reply-To`
header.

Another use case of twt hashes in some twtxt clients is to store which twts
have already been read by the user. Then they can be hidden the next time the
timeline is presented to the user.

## Format

Each twt's hash is calculated using its author, timestamp and contents. The
author feed URL, RFC 3339 formatted timestamp and twt text are joined with line
feeds:

```
<twt author feed URL> "\n"
<twt timestamp in RFC 3339> "\n"
<twt text>
```

This UTF-8 encoded string is Blake2b hashed with 256 bits and Base32 encoded
without padding. After converting to lower case the last seven characters make
up the twt hash.

### Timestamp Format

The twt timestamp must be [RFC 3339](https://tools.ietf.org/html/rfc3339)-formatted,
e.g.:

```
2020-12-13T08:45:23+01:00
2020-12-13T07:45:23Z
```

The time must exactly be truncated or expanded to seconds precision. Any
possible milliseconds must be cut off without any rounding. The seconds part of
minutes precision times must be set to zero.

```
2020-12-13T08:45:23.789+01:00 → 2020-12-13T08:45:23+01:00
2020-12-13T08:45+01:00        → 2020-12-13T08:45:00+01:00
```

All timezones representing UTC must be formatted using the designated Zulu
indicator `Z` rather than the numeric offsets `+00:00` or `-00:00`. If the
timestamp does not explicitly include any timezone information, it must be
assumed to be in UTC.

```
2020-12-13T07:45:23+00:00 → 2020-12-13T07:45:23Z
2020-12-13T07:45:23-00:00 → 2020-12-13T07:45:23Z
2020-12-13T07:45:23       → 2020-12-13T07:45:23Z
```

Other timezone conversations must not be applied. Even though two timestamps
represent the exact point in time in two different time zones, the twt's
original timezone must be used. The following example is illegal:

```
2020-12-13T08:45:23+01:00 → 2020-12-13T07:45:23Z (illegal)
```

As the exact timestamp format will affect the twt hash, these rules must be
followed without any exception.

## Reference Implementation

This section shows reference implementations of this algorithm.

### Go

```
payload := twt.Twter.URL + "\n" + twt.Created.Format(time.RFC3339) + "\n" + twt.Text
sum := blake2b.Sum256([]byte(payload))
encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
hash := strings.ToLower(encoding.EncodeToString(sum[:]))
hash = hash[len(hash)-7:]
```

### Python 3

```
created = twt.created.isoformat().replace("+00:00", "Z")
payload = "%s\n%s\n%s" % (twt.twter.url, created, twt.text)
sum256 = hashlib.blake2b(payload.encode("utf-8"), digest_size=32).digest()
hash = base64.b32encode(sum256).decode("ascii").replace("=", "").lower()[-7:]
```

