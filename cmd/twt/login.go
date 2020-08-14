package main

import (
	"os"
	"path/filepath"

	"github.com/Bowery/prompt"
	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/prologic/twtxt/client"
)

// loginCmd represents the pub command
var loginCmd = &cobra.Command{
	Use:     "login [flags]",
	Aliases: []string{"auth"},
	Short:   "Login and euthenticate to thw twt API",
	Long:    `...`,
	Args:    cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		uri := viper.GetString("uri")
		cli, err := client.NewClient(client.WithURI(uri))
		if err != nil {
			log.WithError(err).Error("error creating client")
			os.Exit(1)
		}

		login(cli, args)
	},
}

func init() {
	RootCmd.AddCommand(loginCmd)
}

func login(cli *client.Client, args []string) {
	username, err := prompt.Basic("Username:", true)
	if err != nil {
		log.WithError(err).Error("error reading username")
		os.Exit(1)
	}

	password, err := prompt.Password("Password:")
	if err != nil {
		log.WithError(err).Error("error reading password")
		os.Exit(1)
	}

	res, err := cli.Login(username, password)
	if err != nil {
		log.WithError(err).Error("error making login request")
		os.Exit(1)
	}

	log.Info("login successful")

	// Find home directory.
	home, err := homedir.Dir()
	if err != nil {
		log.WithError(err).Error("error finding home directory")
		os.Exit(1)
	}

	cli.Config.Token = res.Token
	if err := cli.Config.Save(filepath.Join(home, ".twt.yaml")); err != nil {
		log.WithError(err).Error("error saving config")
		os.Exit(1)
	}
}
