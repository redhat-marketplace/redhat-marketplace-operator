package connect

import (
	"context"
	"fmt"
	"os"
	"time"

	"emperror.dev/errors"
	"github.com/spf13/cobra"
)

var (
	pid, digest, tag string
	timeout          int64
	publishImage     bool
	pidsToDigest     map[string]string
)

var WaitAndPublishCmd = &cobra.Command{
	Use:          "wait-and-publish",
	Short:        "Wait for the set of images to have a passing result and then publish it",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		token := os.Getenv("RH_CONNECT_TOKEN")

		if token == "" {
			return errors.New("no api token provided, set RH_CONNECT_TOKEN")
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*time.Duration(timeout))
		defer cancel()

		results := newContainerResults(pidsToDigest)
		client := NewConnectClient(token)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		start := make(chan bool, 1)
		defer close(start)

		select {
		case start <- true:
		}

		process := func() (bool, error) {
			fmt.Printf("processing containers for tag %s\n", tag)
			results.Process(client, pidsToDigest, tag)

			if results.IsFinished() {
				if results.HasError() {
					results.PrintErrors()
					return false, errors.New("failed to publish all images")
				}

				fmt.Println("publish success")
				return true, nil
			}

			return false, nil
		}

		for {
			var (
				done bool
				err  error
			)

			select {
			case <-start:
				done, err = process()
			case <-ticker.C:
				done, err = process()
			case <-ctx.Done():
				err = errors.New("timed out")
				done = true
			}

			if err != nil {
				return err
			}

			if done {
				return nil
			}
		}
	},
}

func handleErr(err error) {
	if err != nil {
		fmt.Printf("error: %v", err)
		os.Exit(1)
	}
}

func init() {
	WaitAndPublishCmd.Flags().Int64Var(&timeout, "timeout", 15, "timeout in minutes")
	WaitAndPublishCmd.Flags().StringVar(&tag, "tag", "", "tag of the container")
	WaitAndPublishCmd.Flags().BoolVar(&publishImage, "publish", false, "publish")
	WaitAndPublishCmd.Flags().StringToStringVar(&pidsToDigest, "pid", nil, "opsid to digest mapping")
}
