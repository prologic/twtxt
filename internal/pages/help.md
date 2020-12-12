---
title: Help on how to use Twtxt.net
---

# Help

<details>
  <summary>Why do I need an account?</summary>

  You need to have an account so the Twtxt Pod can uniquely identity you
  in a meaningful way and create a personalized timeline based on users
  and feeds you follow as well as allow you to post Twts against your own
  user or create feeds to post as different topics of interested or personas.

  Without creating an account you are limited to reading the pod's local
  user timeline of feeds (_basically all the posts of the users on that pod_)
  but you are unable to participate. You _may_ also be able to view the
  profiles of any user on that Pod **provided** the Pod operator has chosen to
  and configured "open profiles". You are also able to follow a user without an
  account by subscribing to their Atom feed. You can also similarly subscribe to
  the pod's local timeline of users feeds.
</details>

<details>
  <summary>How to create an account?</summary>

  To create an account on {{ .InstanceName }} simply navigate to
  [/register](/register) assuming the operator of this
  instance has left user registration open. There you need to fill
  in a valid Username and Password and optional Email Address
  (_hich is only used for passwrod recovery_).

</details>
<details>
  <summary>How do follow someone?</summary>

  Following someone in the twtxt community is actually rather hard to do
  because of it being decentralised. This means no one company or entity
  controls who posts what, where, why or how on their twtxt feeds.


  Nevertheless following someone on the same instance ({{ .InstanceName }})
  you are on is easy! Simply [/login](/login) and navigate to the
  [/discover](/discover) page to find _public_ posts of
  users on the same instance. This is a good way to discover new users.
</details>
<details>
  <summary>How do I format my posts?</summary>

  The software that powers this pod {{ .InstanceName }} ([jointwt/twtxt](https://github.com/jointwt/twtxt))
  supports what's called [Markdown](https://en.wikipedia.org/wiki/Markdown).
  (_It actually support the full syntax of Markdown really but it is not recommended as twtxt posts are limited  to single lines and length_)

  This means you can format your posts in very simple but powerful ways:

  - Anything that looks like a link is automatically rendered as a click-able link
  - Use `**bold**` or `_italics_` to place emphasis on your parts of your post
  - Use `fixed width` to render text in fixed-width or another style of emphasis
  - Use `[Title](url)` to give your links a nice pretty title
  - Use `![](url)` to link to external images which will be rendered inline with your post

  Of course twtxt is fully Unicode and Emoji capable so  any Emoji you
  can type on your keyboard (_uch as the special keyboard on your iPhone_)
  will also  work nicely 😊

  > **Pro Tip:** Just use the "Formatting Toolbar"
</details>
