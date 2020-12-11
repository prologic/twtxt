package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/jointwt/twtxt/client"
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

		login(cli)
	},
}

func init() {
	RootCmd.AddCommand(loginCmd)
}

func readCredentials() (string, string, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Username: ")
	username, err := reader.ReadString('\n')
	if err != nil {
		log.WithError(err).Error("error reading username")
		return "", "", err
	}

	fmt.Print("Password: ")
	data, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.WithError(err).Error("error reading password")
		return "", "", err
	}
	password := string(data)

	return username, password, nil
}

func login(cli *client.Client) {
	username, password, err := readCredentials()
	if err != nil {
		log.WithError(err).Error("error reading credentials")
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
