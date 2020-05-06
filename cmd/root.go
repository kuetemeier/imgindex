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
	"fmt"
	"os"

	"github.com/spf13/cobra"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"

	log "github.com/sirupsen/logrus"
)

var cfgFile string

var version = "0.1.0"
var appName = "imgmeta"

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

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   appName,
	Short: "A commandline tool written in GO to index image meta data.",
	Long: `A commandline tool written in GO to index image meta data.

	You can e.g. use as a data source in HUGO websites.
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
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	initLog()
	cobra.OnInitialize(initConfig)

	// set config defaults

	rootCmd.Version = version

	viper.SetDefault("verbose", false)
	viper.SetDefault("silent", false)

	/*viper.SetDefault("input", "STDIN")
	viper.SetDefault("output", "STDOUT")
	viper.SetDefault("outputFormat", "rss")*/

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.imgmeta.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.PersistentFlags().BoolP("silent", "s", false, "Silent no output, only errors")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Verbose, print additional informations")

	//rootCmd.PersistentFlags().StringP("input", "i", "STDIN", "Input source (STDIN, File, URL) to read rss feed from")
	//rootCmd.PersistentFlags().StringP("output", "o", "STDOUT", "Output destination to write the new rss stream")
	//rootCmd.PersistentFlags().StringP("outputFormat", "f", "rss", "Output format (rss, atom or json)")

	//rootCmd.PersistentFlags().StringP("count", "c", "-1", "Max numbers of feed entries in the output feed (-1 = infinate/same as input)")

	viper.BindPFlag("silent", rootCmd.PersistentFlags().Lookup("silent"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	//viper.BindPFlag("input", rootCmd.PersistentFlags().Lookup("input"))
	//viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
	//viper.BindPFlag("outputFormat", rootCmd.PersistentFlags().Lookup("outputFormat"))

	//viper.BindPFlag("count", rootCmd.PersistentFlags().Lookup("count"))

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
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".imgmeta" (without extension).
		viper.AddConfigPath("/etc/imgmeta/")
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigName("imgmeta.yml")
	}

	viper.AutomaticEnv() // read in environment variables that match

	//viper.ReadConfig(bytes.NewBuffer(yamlDefaultConfig))

	// If a config file is found, read it in.
	err := viper.ReadInConfig()

	configureLog()

	type tField struct {
		Name     string `mapstructure:"name"`
		Type     string `mapstructure:"type"`
		ID       string `mapstructure:"id"`
		NewField string
	}
	//type tFields []tField
	type tFields struct {
		Collection []tField
	}

	//var fieldList tField
	fieldList := make([]tField, 0)

	//fmt.Println("%v", viper.Get("fields"))

	fmt.Printf("fiels: %v", viper.Get("fields"))
	fmt.Println()

	err = viper.UnmarshalKey("fields", &fieldList)
	if err != nil {
		fmt.Printf("unable to decode into struct, %v", err)
		fmt.Println()
	}

	var f tField
	f = fieldList[0]

	log.Info("Dies ist ein Info Log")

	fmt.Printf("fieldList: %v", fieldList)
	fmt.Println()

	fmt.Printf("f: %v", f.ID)
	fmt.Println()

	os.Exit(0)

	//mt.Println("%v", viper.Get("fields"))

	os.Exit(0)

	if viper.GetBool("silent") && viper.GetBool("verbose") {
		os.Stderr.WriteString("ERROR: \"verbose\" and \"silent\" cannot be activated at the same time.\n")
		os.Exit(-1)
	}

	if viper.GetBool("verbose") {
		log.SetLevel(log.InfoLevel)
		log.Info("Verbose mode: on - using " + appName + " version " + rootCmd.Version)

		if err == nil {
			log.Info("Using config file:", viper.ConfigFileUsed())
		} else {
			log.Info("No config file in use.")
		}

	}

}

// initLog initialize the logging system
func initLog() {
	// Log as JSON instead of the default ASCII formatter.
	//log.SetFormatter(&log.JSONFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	log.SetLevel(log.WarnLevel)
}

// configures the logging system dynamically with custom config settings
func configureLog() {
	//log.SetFormatter(&log.JSONFormatter{})
	log.SetLevel(log.InfoLevel)
}
