package types

type Profile struct {
	Type string

	Username string
	Tagline  string
	URL      string
	TwtURL   string
	BlogsURL string

	Followers map[string]string
	Following map[string]string
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
