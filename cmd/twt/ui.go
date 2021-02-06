package main

import (
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jointwt/twtxt/types"
)

func red(s string) string {
	return fmt.Sprintf("\033[31m%s\033[0m", s)
}
func green(s string) string {
	return fmt.Sprintf("\033[32m%s\033[0m", s)
}
func yellow(s string) string {
	return fmt.Sprintf("\033[33m%s\033[0m", s)
}
func boldgreen(s string) string {
	return fmt.Sprintf("\033[32;1m%s\033[0m", s)
}
func blue(s string) string {
	return fmt.Sprintf("\033[34m%s\033[0m", s)
}

func PrintFollowee(nick, url string) {
	fmt.Printf("> %s @ %s",
		yellow(nick),
		url,
	)
}

func PrintFolloweeRaw(nick, url string) {
	fmt.Printf("%s: %s\n", nick, url)
}

func PrintTwt(twt types.Twt, now time.Time) {
	text := FormatTwt(fmt.Sprintf("%t", twt))

	nick := green(twt.Twter().Nick)
	// TODO: Show mentions
	//if NormalizeURL(twt.Twter.URL) == NormalizeURL(conf.Twturl) {
	//	nick = boldgreen(twt.Twter.Nick)
	//}
	fmt.Printf("> %s (%s)\n%s\n",
		nick,
		humanize.Time(twt.Created()),
		text)
}

func PrintTwtRaw(twt types.Twt) {
	fmt.Printf("%s\t%s\t%t\n",
		twt.Twter().URL,
		twt.Created().Format(time.RFC3339),
		twt)
}
