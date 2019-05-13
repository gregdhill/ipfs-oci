package main

import (
	"os"

	"github.com/containers/buildah/pkg/unshare"
	"github.com/containers/storage/pkg/reexec"
	"github.com/gregdhill/gantry"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	verbose bool
	origin  string
	target  string

	rootCmd = &cobra.Command{
		Use: "gantry",
	}

	pushCmd = &cobra.Command{
		Use:   "push",
		Short: "Push an image to IPFS",
		Run: func(cmd *cobra.Command, args []string) {
			if verbose {
				log.SetLevel(log.DebugLevel)
			}
			logger := log.WithFields(log.Fields{
				"image": origin,
			}).Logger

			store, err := gantry.GetStore()
			if err != nil {
				log.Fatal(err)
			}

			if err = gantry.PushImage(logger, store, origin); err != nil {
				log.Fatal(err)
			}
		},
	}

	pullCmd = &cobra.Command{
		Use:   "pull",
		Short: "Pull an image from IPFS",
		Run: func(cmd *cobra.Command, args []string) {
			if verbose {
				log.SetLevel(log.DebugLevel)
			}
			logger := log.WithFields(log.Fields{
				"image": origin,
			}).Logger

			store, err := gantry.GetStore()
			if err != nil {
				log.Fatal(err)
			}

			if err = gantry.PullImage(logger, store, origin, target); err != nil {
				log.Fatal(err)
			}
		},
	}
)

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Display verbose logging output")
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(pullCmd)
	pushCmd.Flags().StringVarP(&origin, "origin", "o", "", "Store image to push (required)")
	pullCmd.Flags().StringVarP(&origin, "origin", "o", "", "IPFS CID to pull (required)")
	pullCmd.Flags().StringVarP(&target, "target", "t", "", "Store image to write (required)")
	pushCmd.MarkFlagRequired("origin")
	pullCmd.MarkFlagRequired("origin")
	pullCmd.MarkFlagRequired("target")
}

func Execute() {
	// always do this first
	reexec.Init()
	unshare.MaybeReexecUsingUserNamespace(false)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func main() {
	Execute()
}
