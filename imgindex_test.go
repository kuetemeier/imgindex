package main_test

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"

	"github.com/kuetemeier/imgindex/cmd"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ImgIndex", func() {
	Context("without arguments", func() {
		It("should run just fine", func() {
			// The Ginkgo test runner takes over os.Args and fills it with
			// its own flags.  This makes the cobra command arg parsing
			// fail because of unexpected options.  Work around this.

			// Save a copy of os.Args
			origArgs := os.Args[:]

			// Trim os.Args to only the first arg
			os.Args = os.Args[:1] // trim to only the first arg, which is the command itself

			b := bytes.NewBufferString("")
			cmd.RootCmd.SetOut(b)
			log.SetOutput(b)

			// Run the command which parses os.Args
			err := cmd.RootCmd.Execute()

			// Restore os.Args
			os.Args = origArgs[:]

			Expect(err).Should(BeNil())

			out, err := ioutil.ReadAll(b)

			Expect(err).Should(BeNil())

			Expect(string(out)).Should(MatchRegexp(`.*`))
		})
	})

	Context("with 'help' argument", func() {

		It("should show a help message", func() {

			// Save a copy of os.Args
			origArgs := os.Args[:]

			// Trim os.Args to only the first arg
			//os.Args = os.Args[:1] // trim to only the first arg, which is the command itself
			os.Args = append(os.Args[:1], "help")

			b := bytes.NewBufferString("")
			cmd.RootCmd.SetOut(b)

			// Run the command which parses os.Args
			err := cmd.RootCmd.Execute()

			// Restore os.Args
			os.Args = origArgs[:]

			Expect(err).Should(BeNil())

			out, err := ioutil.ReadAll(b)

			Expect(err).Should(BeNil())

			Expect(string(out)).Should(MatchRegexp(`.*Usage:.*`))
		})
	})

	Context("with 'version' argument", func() {

		It("should show a version message", func() {

			// Save a copy of os.Args
			origArgs := os.Args[:]

			// Trim os.Args to only the first arg
			//os.Args = os.Args[:1] // trim to only the first arg, which is the command itself
			os.Args = append(os.Args[:1], "version")

			b := bytes.NewBufferString("")
			cmd.RootCmd.SetOut(b)

			// Run the command which parses os.Args
			err := cmd.RootCmd.Execute()

			// Restore os.Args
			os.Args = origArgs[:]

			Expect(err).Should(BeNil())

			out, err := ioutil.ReadAll(b)

			Expect(err).Should(BeNil())

			Expect(string(out)).Should(MatchRegexp(".*" + cmd.AppName + " version " + cmd.RootCmd.Version + ".*"))
		})
	})

})
