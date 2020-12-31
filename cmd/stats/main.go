package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

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
	fmt.Println("tags: ", len(twt.Twts().Tags()))
	for tag, count := range twt.Twts().TagCount() {
		fmt.Println("   ", tag, ": ", count)
	}

	fmt.Println("mentions: ", len(twt.Twts().Mentions()))
	for m, count := range twt.Twts().MentionCount() {
		fmt.Println("   ", m, ": ", count)
	}

	fmt.Println("links: ", len(twt.Twts().Links()))
	for m, count := range twt.Twts().LinkCount() {
		fmt.Println("   ", m, ": ", count)
	}
}
