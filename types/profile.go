package types

// Profile represents a user/feed profile
type Profile struct {
	Type string

	Username  string
	Tagline   string
	URL       string
	TwtURL    string
	BlogsURL  string
	AvatarURL string

	// `true` if the User viewing the Profile has muted this user/feed
	Muted bool

	// `true` if the User viewing the Profile has follows this user/feed
	Follows bool

	// `true` if user/feed follows the User viewing the Profile.
	FollowedBy bool

	Bookmarks map[string]string
	Followers map[string]string
	Following map[string]string

	// `true` if the User viewing the Profile has permissions to show the
	// bookmarks/followers/followings of this user/feed
	ShowBookmarks bool
	ShowFollowers bool
	ShowFollowing bool
}

type Link struct {
	Href string
	Rel  string
}

type Alternative struct {
	Type  string
	Title string
	URL   string
}

type Alternatives []Alternative
type Links []Link
