package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/prologic/go-gopher"

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

	case "gopher":
		res, err := gopher.Get(url.String())
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		if res.Body == nil {
			fmt.Printf("Error: body is empty %v", res.Type)
			os.Exit(1)
		}
		defer res.Body.Close()

		run(res.Body)

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
		fmt.Printf("  % -30s = %s\n", c.Nick, c.URL)
	}

	fmt.Println("twts: ", len(twt.Twts()))

	fmt.Printf("days of week:\n%v\n", daysOfWeek(twt.Twts()))

	fmt.Println("tags: ", len(twt.Twts().Tags()))
	fmt.Println(getTags(twt.Twts().Tags()))

	fmt.Println("mentions: ", len(twt.Twts().Mentions()))
	fmt.Println(getMentions(twt.Twts(), twt.Info().Followers()))

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

	for _, twt := range twt.Twts() {
		fmt.Print(twt.(*lextwt.Twt).FilePos(), "\t", twt)
	}

}

func daysOfWeek(twts types.Twts) stats {
	s := make(map[string]int)

	for _, twt := range twts {
		s[fmt.Sprint(twt.Created().Format("tz-Z0700"))]++
		s[fmt.Sprint(twt.Created().Format("dow-Mon"))]++
		s[fmt.Sprint(twt.Created().Format("year-2006"))]++
		s[fmt.Sprint(twt.Created().Format("day-2006-01-02"))]++
	}

	var lis stats
	for k, v := range s {
		lis = append(lis, stat{v, k})
	}
	return lis
}

func getMentions(twts types.Twts, follows []types.Twter) stats {
	counts := make(map[string]int)
	for _, m := range twts.Mentions() {
		t := m.Twter()
		counts[fmt.Sprint(t.Nick, "\t", t.URL)]++
	}

	lis := make(stats, 0, len(counts))
	for name, count := range counts {
		lis = append(lis, stat{count, name})
	}

	return lis
}

func getTags(twts types.TagList) stats {
	counts := make(map[string]int)
	for _, m := range twts {
		counts[fmt.Sprint(m.Text(), "\t", m.Target())]++
	}

	lis := make(stats, 0, len(counts))
	for name, count := range counts {
		lis = append(lis, stat{count, name})
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
