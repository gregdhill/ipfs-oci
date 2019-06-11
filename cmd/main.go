package main

import (
	"fmt"
	"os"

	"golang.org/x/net/context"

	"github.com/containers/buildah/pkg/unshare"
	gantry "github.com/gregdhill/gantry/image"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	debug bool
	image string
	name  string
	cid   string

	rootCmd = &cobra.Command{
		Use: "gantry",
	}

	pushCmd = &cobra.Command{
		Use:   "push",
		Short: "Push an image to IPFS",
		RunE: func(cmd *cobra.Command, args []string) error {
			if debug {
				log.SetLevel(log.DebugLevel)
			}

			logger := log.WithFields(log.Fields{
				"image": image,
			})

			store, err := gantry.GetStore()
			if err != nil {
				log.Fatal(err)
			}

			ctx := context.Background()
			api, err := gantry.GetAPI(ctx)
			if err != nil {
				return err
			}

			return gantry.PushImage(ctx, logger, store, api, image)
		},
	}

	pullCmd = &cobra.Command{
		Use:   "pull",
		Short: "Pull an image from IPFS",
		RunE: func(cmd *cobra.Command, args []string) error {
			if debug {
				log.SetLevel(log.DebugLevel)
			}

			logger := log.WithFields(log.Fields{
				"image": image,
			})

			store, err := gantry.GetStore()
			if err != nil {
				return err
			}

			ctx := context.Background()
			api, err := gantry.GetAPI(ctx)
			if err != nil {
				return err
			}

			if name != "" {
				cid, err = gantry.ResolveName(ctx, api, name)
				if err != nil {
					return err
				}
			} else if cid == "" {
				return fmt.Errorf("require cid or name to resolve")
			}

			return gantry.PullImage(ctx, logger, store, api, cid, image)
		},
	}
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Display debug logging output")

	rootCmd.AddCommand(pushCmd)
	pushCmd.Flags().StringVarP(&image, "image", "i", "", "Image to publish (required)")
	pushCmd.MarkFlagRequired("image")

	rootCmd.AddCommand(pullCmd)
	pullCmd.Flags().StringVarP(&image, "image", "i", "", "Image to write (required)")
	pullCmd.Flags().StringVarP(&cid, "cid", "c", "", "Content identifier to pull (required)")
	pullCmd.Flags().StringVarP(&name, "name", "n", "", "Mutable link to resolve (required)")
	pullCmd.MarkFlagRequired("image")
	pullCmd.MarkFlagRequired("cid")
}

func main() {
	// always do this first
	unshare.MaybeReexecUsingUserNamespace(false)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
