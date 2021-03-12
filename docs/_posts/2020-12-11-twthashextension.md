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
author feed URL (see below, it is not necessarily identical to the URL which is
being retrieved), RFC 3339 formatted timestamp and twt text are joined with line
feeds:

```
<twt author feed URL> "\n"
<twt timestamp in RFC 3339> "\n"
<twt text>
```

This UTF-8 encoded string is Blake2b hashed with 256 bits and Base32 encoded
without padding. After converting to lower case the last seven characters make
up the twt hash.

### Choosing the feed URL

This addresses setups where the same feed is served over multiple protocols
(HTTP, HTTPS, Gopher, ...).

Feeds can include metadata at the beginning. This includes one or more `url`
fields:

```
# nick = cathy
# url  = https://cathy.example.com/twtxt.txt
# url  = http://cathy.example.com/twtxt.txt
# url  = gopher://cathy.example.com/0/twtxt.txt
2020-10-11T10:40:48+02:00	hello world
...
```

If `url` fields are present, the first one must be used for hashing. If none are
present, then the URL which was used to retrieve the feed must be used.

Users are advised to not change the first one of their `url`s. If they move
their feed to a new URL, they should add this new URL as a new `url` field.

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

```go
payload := twt.Twter.URL + "\n" + twt.Created.Format(time.RFC3339) + "\n" + twt.Text
sum := blake2b.Sum256([]byte(payload))
encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
hash := strings.ToLower(encoding.EncodeToString(sum[:]))
hash = hash[len(hash)-7:]
```

### Python 3

```python
created = twt.created.isoformat().replace("+00:00", "Z")
payload = "%s\n%s\n%s" % (twt.twter.url, created, twt.text)
sum256 = hashlib.blake2b(payload.encode("utf-8"), digest_size=32).digest()
hash = base64.b32encode(sum256).decode("ascii").replace("=", "").lower()[-7:]
```

### Node.js

```javascript
const b32encode = require('base32-encode');
const blake2 = require('blake2');
const { DateTime } = require('luxon');

function base32(payload) {
  return b32encode(Buffer.from(payload), 'RFC3548', { padding: false });
}

function blake2b256(payload) {
  return blake2.createHash('blake2b', { digestLength: 32 })
    .update(Buffer.from(payload))
    .digest();
}

function formatRFC3339(text) {
  return DateTime.fromISO(text, { setZone: true, zone: 'utc' })
    .toFormat("yyyy-MM-dd'T'HH:mm:ssZZ")
    .replace(/\+00:00$/, 'Z');
}

const created = formatRFC3339(twt.created);
const payload = [twt.twter.url, created, twt.content].join('\n');
const hash = base32(blake2b256(payload)).toLowerCase().slice(-7);
```
