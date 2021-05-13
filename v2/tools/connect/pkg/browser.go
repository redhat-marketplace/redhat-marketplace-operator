// Copyright 2021 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pkg

import (
	"context"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"encoding/csv"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"github.com/spf13/cobra"
)

var (
	images             []string
	username, password string
	isOperatorManifest bool
)

var PublishCommand = &cobra.Command{
	Use:   "publish",
	Short: "Publish's the image on PC.",
	RunE: func(cmd *cobra.Command, args []string) error {
		log.SetOutput(os.Stdout)
		csvStr := strings.Join(append([]string{"project_url,sha,tag"}, images...), "\n")
		csvRows, err := csv.NewReader(strings.NewReader(csvStr)).ReadAll()

		if err != nil {
			return err
		}

		images := []*imageStruct{}
		for _, row := range csvRows[1:] {
			image := &imageStruct{}
			image.FromCSV(row)
			images = append(images, image)
			log.Printf("image passed %+v\n", image)
		}

		web := &connectWebsite{
			Username: username,
			Password: password,
		}

		log.Println("starting")

		web.Start()
		defer web.Stop()

		err = web.Login(web.Ctx, username, password)

		if err != nil {
			log.Fatal(err)
			return err
		}

		start := make(chan bool, 1)
		defer close(start)

		select {
		case start <- true:
		}

		published := map[string]interface{}{}

		process := func() (chan bool) {
			boolChan := make(chan bool)

			go func() {
				defer close(boolChan)

				for {
					for _, image := range images {

						if _, ok := published[image.SHA]; ok {
							log.Println(image.SHA, "is already published")
							continue
						}

						func() {
							localCtx, cancelTimer := context.WithTimeout(web.Ctx, 1*time.Minute)
							defer cancelTimer()

							isPublished, err := web.Publish(localCtx, image)

							if isPublished {
								published[image.SHA] = nil
							}

							if err != nil {
								log.Println("failed to publish", err)
							}
						}()
					}

					foundAll := true
					for _, image := range images {
						_, ok := published[image.SHA]
						foundAll = foundAll && ok
					}

					if foundAll {
						log.Println("finished publishing")
						boolChan <- true
						return
					}

					time.Sleep(30 * time.Second)
				}
				return
			}()

			return boolChan
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		doneChan := process()

		select {
		case <-web.Ctx.Done():
			log.Println("context cancelled")
			return errors.New("context cancelled")
		case <-doneChan:
			log.Println("done")
		case <-ctx.Done():
			err = errors.New("timed out")
			return err
		}

		return nil
	},
}

func init() {
	PublishCommand.Flags().StringArrayVar(&images, "images", []string{}, "list comma seperated image vars to publish")
	PublishCommand.Flags().StringVar(&username, "username", os.Getenv("RH_USER"), "username for PC")
	PublishCommand.Flags().StringVar(&password, "password", os.Getenv("RH_PASSWORD"), "password for PC")
	PublishCommand.Flags().BoolVar(&isOperatorManifest, "is-operator-manifest", false, "if the images sent are operator bundles")
}

type imageStruct struct {
	ProjectURL string `csv:"project_url"`
	SHA        string `csv:"sha"`
	Tag        string `csv:"tag"`
}

func (img *imageStruct) FromCSV(strs []string) error {
	if len(strs) != 3 {
		return errors.New("incorrect length")
	}

	i := 0
	img.ProjectURL = strs[i]
	i = i + 1
	img.SHA = strs[i]
	i = i + 1
	img.Tag = strs[i]

	return nil
}

func getChromeContext(dir string) (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.UserDataDir(dir),
	)

	allocCtx, cancel1 := chromedp.NewExecAllocator(context.Background(), opts...)

	// also set up a custom logger
	taskCtx, cancel2 := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))

	return taskCtx, func() {
		defer cancel1()
		defer cancel2()
	}
}

type connectWebsite struct {
	Ctx                context.Context
	stop               chan struct{}
	Username, Password string

	directory string
}

var onlyOneSignalHandler = make(chan struct{})
var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

func (c *connectWebsite) Start() error {
	close(onlyOneSignalHandler) // panics when called twice
	// also set up a custom logger
	taskCtx, cancel := getChromeContext(c.directory)
	c.Ctx = taskCtx

	c.stop = make(chan struct{})
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, shutdownSignals...)
	go func() {
		<-ch
		log.Println("shutdown signal received")
		close(c.stop)
		<-ch
		log.Println("second shutdown signal received, killing")
		os.Exit(1) // second signal. Exit directly.
	}()

	go func() {
		<-c.stop
		cancel()
	}()

	return nil
}

func (c *connectWebsite) Stop() {
	if c.stop == nil {
		return
	}
	close(c.stop)
}

func (c *connectWebsite) Login(inCtx context.Context, username, password string) error {
	// create a timeout

	err := chromedp.Run(inCtx,
		chromedp.Navigate(`https://connect.redhat.com/saml_login`),
		chromedp.WaitVisible(`body > div.layout-container > footer`),
		chromedp.SendKeys(`//input[@name="username"]`, username),
		chromedp.Click(`//button[@id="login-show-step2"]`),
		chromedp.WaitVisible(`//input[@name="password"]`),
		chromedp.SetValue(`//input[@name="password"]`, password),
		chromedp.Submit(`//input[@name="password"]`),
		chromedp.WaitEnabled(`div.region-page-bottom`),
	)

	if err != nil {
		return err
	}

	log.Println("logged in")

	return nil
}

func (c *connectWebsite) Publish(ctx context.Context, image *imageStruct) (bool, error) {
	publishSha := image.SHA
	url := image.ProjectURL

	nodes := []*cdp.Node{}

	log.Println("navigating to", url)

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady(`table[aria-label="Images List"]`, chromedp.ByQuery),
	)

	if err != nil {
		return false, err
	}

	log.Println("url ready", url)

	err = chromedp.Run(ctx,
		chromedp.Nodes(`table[aria-label="Images List"] > tbody`, &nodes, chromedp.ByQueryAll),
	)

	if err != nil {
		return false, err
	}

	for _, node := range nodes {
		var sha, tag string
		localNode := node
		err := chromedp.Run(ctx,
			chromedp.Text(`td[data-label="Image"] > span > span`, &sha, chromedp.ByQuery, chromedp.FromNode(localNode)),
			chromedp.Text(`td[data-label="Tags"] > span > span`, &tag, chromedp.ByQuery, chromedp.FromNode(localNode)))

		if err != nil {
			return false, err
		}

		if sha != "" && sha == publishSha || isOperatorManifest && tag != "" && strings.HasPrefix(tag, image.Tag) {
			publishStatus, err := func() (string, error) {
				log.Println("checking if we need to publish", sha, tag)

				visibilitySel := `td[data-label="Visibility"]`

				var status string

				err := chromedp.Run(ctx,
					chromedp.Text(visibilitySel, &status, chromedp.ByQuery, chromedp.FromNode(localNode)),
				)

				if err != nil {
					log.Println(err)
					return "", err
				}

				return status, nil
			}()

			if err != nil {
				log.Printf("failed to get publish status")
				return false, err
			}

			if publishStatus == "Published" {
				log.Println("sha is published", sha)
				return true, nil
			}

			err = func() error {
				log.Println("publishing", sha)

				toggleSel := `button[id*="expandable-toggle"]`
				publishSel := `button.rhc-images__list-container__publish-btn`
				publishFinalSel := `div[id*="pf-modal-part"] > footer > button[data-testid="publish-tag-modal-ok"]`

				err := chromedp.Run(ctx,
					chromedp.Click(toggleSel, chromedp.ByQuery, chromedp.FromNode(localNode)),
					chromedp.ActionFunc(logAction("toggle")),
					chromedp.WaitVisible(publishSel, chromedp.ByQuery, chromedp.FromNode(localNode)),
					chromedp.Click(publishSel, chromedp.ByQuery, chromedp.FromNode(localNode)),
					chromedp.ActionFunc(logAction("publish")),
					chromedp.WaitVisible(publishFinalSel),
					chromedp.Click(publishFinalSel, chromedp.ByQuery),
					chromedp.ActionFunc(logAction("publish final")),
					chromedp.WaitNotPresent(publishFinalSel, chromedp.ByQuery),
				)

				if err != nil {
					log.Println(err)
					return err
				}

				return nil
			}()

			if err != nil {
				return false, err
			}

			log.Println("publishing finished", sha, tag)
			return true, nil
		}
	}

	return false, nil
}

func logAction(msg string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		log.Println(msg)
		return nil
	}
}

func captureScreenshot(name string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		var buf []byte
		err := chromedp.Run(ctx, chromedp.FullScreenshot(&buf, 90))
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(name, buf, 0644); err != nil {
			log.Fatal(err)
		}
		return nil
	}
}
