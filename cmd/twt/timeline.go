package main

import (
	"fmt"
	"os"
	"sort"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jointwt/twtxt/client"
)

// timelineCmd represents the pub command
var timelineCmd = &cobra.Command{
	Use:     "timeline [flags]",
	Aliases: []string{"view", "show", "events"},
	Short:   "Display your timeline",
	Long:    `...`,
	//Args:    cobra.NArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		uri := viper.GetString("uri")
		token := viper.GetString("token")
		cli, err := client.NewClient(
			client.WithURI(uri),
			client.WithToken(token),
		)
		if err != nil {
			log.WithError(err).Error("error creating client")
			os.Exit(1)
		}

		timeline(cli, args)
	},
}

func init() {
	RootCmd.AddCommand(timelineCmd)
}

func timeline(cli *client.Client, args []string) {
	// TODO: How do we get more pages?
	res, err := cli.Timeline(0)
	if err != nil {
		log.WithError(err).Error("error retrieving timeline")
		os.Exit(1)
	}

	sort.Sort(sort.Reverse(res.Twts))

	for _, twt := range res.Twts {
		PrintTwt(twt, time.Now())
		fmt.Println()
	}
}
