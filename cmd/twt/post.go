package main

import (
	"io/ioutil"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jointwt/twtxt/client"
)

// postCmd represents the pub command
var postCmd = &cobra.Command{
	Use:     "post [flags]",
	Aliases: []string{"tweet", "twt", "new"},
	Short:   "Post a Twt to a Twtxt Pod",
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

		post(cli, args)
	},
}

func init() {
	RootCmd.AddCommand(postCmd)
}

func post(cli *client.Client, args []string) {
	text := strings.Join(args, " ")

	if text == "" {
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.WithError(err).Error("error reading text from stdin")
			os.Exit(1)
		}
		text = string(data)
	}

	if text == "" {
		log.Error("no text provided")
		os.Exit(1)
	}

	log.Info("posting twt...")

	_, err := cli.Post(text)
	if err != nil {
		log.WithError(err).Error("error making post")
		os.Exit(1)
	}

	log.Info("post successful")
}
