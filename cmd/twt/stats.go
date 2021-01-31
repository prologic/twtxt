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
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/jointwt/twtxt/types"
	"github.com/jointwt/twtxt/types/lextwt"
)

// statsCmd represents the stats command
var statsCmd = &cobra.Command{
	Use:     "stats [flags] <url|file>",
	Aliases: []string{},
	Short:   "Parses and performs statistical analytis on a Twtxt feed given a URL or local file",
	Long:    `...`,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runStats(args)
	},
}

func init() {
	RootCmd.AddCommand(statsCmd)
}

func runStats(args []string) {
	url, err := url.Parse(args[0])
	if err != nil {
		log.WithError(err).Error("error parsing url")
		os.Exit(2)
	}

	log.Debugf("Reading: %s", url)

	switch url.Scheme {
	case "", "file":
		f, err := os.Open(url.Path)
		if err != nil {
			log.WithError(err).Error("error reading file feed")
			os.Exit(2)
		}
		defer f.Close()

		doStats(f)
	case "http", "https":
		f, err := http.Get(url.String())
		if err != nil {
			log.WithError(err).Error("error reading HTTP feed")
			os.Exit(2)
		}
		defer f.Body.Close()

		doStats(f.Body)
	case "gopher":
		res, err := gopher.Get(url.String())
		if err != nil {
			log.WithError(err).Error("error reading Gopher feed")
			os.Exit(2)
		}
		defer res.Body.Close()

		doStats(res.Body)
	}
}

func doStats(r io.Reader) {
	log.Debug("Parsing file...")

	twt, err := lextwt.ParseFile(r, types.NilTwt.Twter())
	if err != nil {
		log.WithError(err).Error("error parsing feed")
		os.Exit(2)
	}
	log.Debug("Complete!")

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
