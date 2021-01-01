package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/jointwt/twtxt/types"
	"github.com/jointwt/twtxt/types/lextwt"
)

func main() {
	if len(os.Args) == 0 {
		fmt.Println("Usage: stats <url>")
		os.Exit(1)
	}

	url, err := url.Parse(os.Args[1])
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Reading: ", url)

	switch url.Scheme {
	case "", "file":
		f, err := os.Open(url.Path)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		defer f.Close()

		run(f)

	case "http", "https":
		f, err := http.Get(url.String())
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		defer f.Body.Close()

		run(f.Body)
	}
}

func run(r io.Reader) {
	fmt.Println("Parsing file...")

	twt, err := lextwt.ParseFile(r, types.NilTwt.Twter())
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Complete!")
	fmt.Println(twt.Info())

	twter := twt.Twter()
	m := lextwt.NewMention(twter.Nick, twter.URL)
	fmt.Printf("twter: %s@%s url: %s\n", m.Name(), m.Domain(), m.URL())

	fmt.Println("metadata:")
	for _, c := range twt.Info().GetAll("") {
		fmt.Printf("  %s = %s\n", c.Key(), c.Value())
	}

	fmt.Println("followers:")
	for _, c := range twt.Info().Followers() {
		fmt.Printf("  % -30s = %s\n", c.Key(), c.Value())
	}

	fmt.Println("twts: ", len(twt.Twts()))

	fmt.Printf("days of week:\n%v\n", daysOfWeek(twt.Twts()))

	fmt.Println("tags: ", len(twt.Twts().Tags()))
	var tags stats
	for tag, count := range twt.Twts().TagCount() {
		tags = append(tags, stat{count, tag})
	}
	fmt.Println(tags)

	fmt.Println("mentions: ", len(twt.Twts().Mentions()))
	var mentions stats
	for mention, count := range twt.Twts().MentionCount() {
		mentions = append(mentions, stat{count, mention})
	}
	fmt.Println(mentions)

	fmt.Println("subjects: ", len(twt.Twts().Subjects()))
	var subjects stats
	for subject, count := range twt.Twts().SubjectCount() {
		subjects = append(subjects, stat{count, subject})
	}
	fmt.Println(subjects)

	fmt.Println("links: ", len(twt.Twts().Links()))
	var links stats
	for link, count := range twt.Twts().LinkCount() {
		links = append(links, stat{count, link})
	}
	fmt.Println(links)

}

func daysOfWeek(twts types.Twts) stats {
	s := make(map[string]int)

	for _, twt := range twts {
		s[fmt.Sprint(twt.Created().Format("tz-Z0700"))]++
		s[fmt.Sprint(twt.Created().Format("dow-Mon"))]++
		s[fmt.Sprint(twt.Created().Format("2006-01-02"))]++
	}

	var lis stats
	for k, v := range s {
		lis = append(lis, stat{v, k})
	}
	return lis
}

type stat struct {
	count int
	text  string
}

func (s stat) String() string {
	return fmt.Sprintf("  %v : %v\n", s.count, s.text)
}

func (s stats) Len() int {
	return len(s)
}
func (s stats) Less(i, j int) bool {
	return s[i].count > s[j].count
}
func (s stats) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

type stats []stat

func (s stats) String() string {
	var b strings.Builder
	sort.Sort(s)
	for _, line := range s {
		b.WriteString(line.String())
	}
	return b.String()
}
