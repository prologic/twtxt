package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jointwt/twtxt"
	client "github.com/prologic/twtxt/client"
)

var configFile string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:     "twt",
	Version: twtxt.FullVersion(),
	Short:   "Command-line client for twtxt",
	Long:    `...`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// set logging level
		if viper.GetBool("debug") {
			log.SetLevel(log.DebugLevel)
		} else {
			log.SetLevel(log.InfoLevel)
		}
	},
}

// Execute adds all child commands to the root command
// and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		log.WithError(err).Error("error executing command")
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVar(
		&configFile, "config", "",
		"config file (default: $HOME/.twt.yaml)",
	)

	RootCmd.PersistentFlags().BoolP(
		"debug", "d", false,
		"Enable debug logging",
	)

	RootCmd.PersistentFlags().StringP(
		"uri", "u", client.DefaultURI,
		"twt API endpoint URI to connect to",
	)

	RootCmd.PersistentFlags().StringP(
		"token", "t", "$TWT_TOKEN",
		"twt API token to use to authenticate to endpoints",
	)

	viper.BindPFlag("uri", RootCmd.PersistentFlags().Lookup("uri"))
	viper.SetDefault("uri", client.DefaultURI)

	viper.BindPFlag("token", RootCmd.PersistentFlags().Lookup("token"))
	viper.SetDefault("token", os.Getenv("TWT_TOKEN"))

	viper.BindPFlag("debug", RootCmd.PersistentFlags().Lookup("debug"))
	viper.SetDefault("debug", false)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if configFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(configFile)

		// If a config file is found, read it in.
		if err := viper.ReadInConfig(); err != nil {
			log.WithError(err).Errorf("error loading config file")
			os.Exit(1)
		}
		log.Infof("Using config file: %s", viper.ConfigFileUsed())
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		viper.AddConfigPath(home)
		viper.SetConfigName(".twt")

		// If a config file is found, read it in.
		if err := viper.ReadInConfig(); err != nil {
			log.WithError(err).Warn("error loading config file")
		} else {
			log.Infof("Using config file: %s", viper.ConfigFileUsed())
		}
	}

	// from the environment
	viper.SetEnvPrefix("TWT")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv() // read in environment variables that match
}
