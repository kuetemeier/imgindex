/*
Copyright © 2020 Jörg Kütemeier <joerg@kuetemeier.de>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package cmd holds all commands.
package cmd

import (
	"os"

	"github.com/spf13/cobra"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"

	log "github.com/sirupsen/logrus"
)

var cfgFile string

// Application version as in RootCmd.version
const version = "0.1.0"

// AppName is the string representation of the application Name
const AppName = "ImgIndex"

// any approach to require this configuration into your program.
var yamlDefaultConfig = []byte(`
fields:
-
  name: filename
  type: core
  id: filename
-
  name: filenameRel
  type: core
  id: filenameRelative
-
  name: version
  type: core
  id: version
-
  test: no other fields
Hacker: true
name: steve
hobbies:
- skateboarding
- snowboarding
- go
clothing:
  jacket: leather
  trousers: denim
age: 35
eyes : brown
beard: true
`)

type tField struct {
	Name     string `mapstructure:"name"`
	Type     string `mapstructure:"type"`
	ID       string `mapstructure:"id"`
	NewField string
}

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   AppName,
	Short: "ImgIndex - Image meta data (EXIF, IPTC, XMP) crawler and indexer (to JSON), written in GO.",
	Long: `ImgIndex - Image meta data (EXIF, IPTC, XMP) crawler and indexer (to JSON), written in GO.

	It collects given (configured) fields of meta data from images stored in a directory
	structure and writes them to a central JSON files.

	You can use this JSON file e.g. as a data source in HUGO websites.
	`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		indexCmd.Run(cmd, args)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

func init() {
	initLog()
	cobra.OnInitialize(initConfig)

	// set config defaults

	RootCmd.Version = version

	viper.SetDefault("fields", []tField{})

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	RootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.imgindex.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.

	viper.SetDefault("verbose", false)
	RootCmd.PersistentFlags().BoolP("verbose", "v", false, "Verbose, print additional informations")
	viper.BindPFlag("verbose", RootCmd.PersistentFlags().Lookup("verbose"))

	viper.SetDefault("silent", false)
	RootCmd.PersistentFlags().BoolP("silent", "s", false, "Silent no output, only errors")
	viper.BindPFlag("silent", RootCmd.PersistentFlags().Lookup("silent"))

	viper.SetDefault("debug", false)
	RootCmd.PersistentFlags().BoolP("debug", "d", false, "Debug, print additional and debug informations")
	viper.BindPFlag("debug", RootCmd.PersistentFlags().Lookup("debug"))

}

// initConfig reads in config file and ENV variables if set.
func initConfig() {

	viper.SetConfigType("yaml")

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".imgindex" (without extension).
		viper.AddConfigPath("/etc/imgindex/")
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigName("imgindex.yml")
	}

	viper.AutomaticEnv() // read in environment variables that match

	//viper.ReadConfig(bytes.NewBuffer(yamlDefaultConfig))

	// If a config file is found, read it in.
	err := viper.ReadInConfig()

	configureLog()

	if viper.GetBool("silent") && viper.GetBool("verbose") {
		log.Fatal("ERROR: \"verbose\" and \"silent\" config options cannot be activated at the same time.\n")
		os.Exit(-1)
	}

	if viper.GetBool("silent") && viper.GetBool("debug") {
		log.Fatal("ERROR: \"debug\" and \"silent\" config options cannot be activated at the same time.\n")
		os.Exit(-1)
	}
	if viper.GetBool("verbose") || viper.GetBool("debug") {
		log.SetLevel(log.InfoLevel)
		log.Info("Verbose mode: on - using " + AppName + " version " + RootCmd.Version)

		if err == nil {
			log.Info("Using config file:", viper.ConfigFileUsed())
		} else {
			log.Info("No config file in use.")
		}
	}

	if viper.GetBool("debug") {
		log.SetLevel(log.DebugLevel)
		//log.SetReportCaller(true)
		log.Debug("Debugging Log-Level activated.")
	}

	processConfig()

}

// initLog initialize the logging system
func initLog() {
	// Log as JSON instead of the default ASCII formatter.
	//log.SetFormatter(&log.JSONFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	//log.SetOutput(os.Stdout)
	log.SetOutput(RootCmd.OutOrStdout())

	// Only log the warning severity or above.
	log.SetLevel(log.WarnLevel)
}

// configures the logging system dynamically with custom config settings
func configureLog() {
	//log.SetFormatter(&log.JSONFormatter{})

	// double - here and in initLog - configureLog is called AFTER the init process
	log.SetOutput(RootCmd.OutOrStdout())
	log.SetLevel(log.InfoLevel)
}

func processConfig() {

	//type tFields []tField
	type tFields struct {
		Collection []tField
	}

	//var fieldList tField
	fieldList := make([]tField, 0)

	log.Debugf("fiels: %v", viper.Get("fields"))
	log.Debugln()

	err := viper.UnmarshalKey("fields", &fieldList)
	if err != nil {
		log.Fatal("unable to decode 'fields' configuration into struct:", err)
	}

	if len(fieldList) > 0 {
		var f tField
		f = fieldList[0]

		log.Debug("fieldList:", fieldList)
		log.Debug("f: %v", f.ID)
	}
}
