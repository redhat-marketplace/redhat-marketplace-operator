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
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"encoding/csv"
	"encoding/json"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"github.com/spf13/cobra"
	"github.com/thediveo/enumflag"
)

var (
	images             []string
	username, password string
	isOperatorManifest bool
	verbose            bool

	pageSize ListSize = ListSize20
	urls     []string
)

var GetPublishStatusCommand = &cobra.Command{
	Use:   "status",
	Short: "Retrieves the current status of publish",
	RunE: func(cmd *cobra.Command, args []string) error {

		if !verbose {
			log.SetOutput(ioutil.Discard)
		} else {
			log.SetOutput(cmd.OutOrStderr())
		}

		web := &ConnectWebsite{
			Username: username,
			Password: password,
		}

		webCtx, cancel := web.Start()
		defer cancel()

		ctx, cancel := context.WithTimeout(webCtx, 4*time.Minute)
		defer cancel()

		err := web.Login(ctx, username, password)

		if err != nil {
			return err
		}

		statuses := []*ImagePublishStatus{}

		for _, url := range urls {
			results, err := web.GetAllStatuses(ctx, url)
			if err != nil {
				return err
			}
			statuses = append(statuses, results...)
		}

		data, err := json.Marshal(statuses)

		if err != nil {
			return err
		}

		cmd.OutOrStdout().Write(data)
		return nil
	},
}

var PublishCommand = &cobra.Command{
	Use:   "publish",
	Short: "Publish's the image on PC.",
	RunE: func(cmd *cobra.Command, args []string) error {
		log.SetOutput(cmd.OutOrStdout())

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

		web := &ConnectWebsite{
			Username: username,
			Password: password,
		}

		log.Println("starting")

		webCtx, cancel := web.Start()
		defer cancel()

		overallContext, ocancel := context.WithTimeout(webCtx, 10*time.Minute)
		defer ocancel()

		if err != nil {
			log.Println("failed to start web", err)
			return err
		}

		err = func() error {
			return web.Login(overallContext, username, password)
		}()

		if err != nil {
			log.Println("failed to login", err)
			return err
		}

		start := make(chan bool, 1)
		defer close(start)

		select {
		case start <- true:
		}

		published := map[string]interface{}{}

		for _, image := range images {

			if _, ok := published[image.SHA]; ok {
				log.Println(image.SHA, "is already published")
				continue
			}

			func() {
				ctx, lcancel := context.WithTimeout(overallContext, 1*time.Minute)
				defer lcancel()

				isPublished, err := web.Publish(ctx, image)

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
			return nil
		}

		return errors.New("failed to publish all the images")
	},
}

type ListSize enumflag.Flag

const (
	ListSize10 ListSize = iota
	ListSize20
	ListSize50
	ListSize100
)

var ListSizeIds = map[ListSize][]string{
	ListSize10:  {"10", "ten"},
	ListSize20:  {"20", "twenty"},
	ListSize50:  {"50", "fifth"},
	ListSize100: {"100", "hundred"},
}

func (l ListSize) String() string {
	return ListSizeIds[l][0]
}

func init() {
	PublishCommand.Flags().StringArrayVar(&images, "images", []string{}, "list comma seperated image vars to publish")
	PublishCommand.Flags().StringVar(&username, "username", os.Getenv("RH_USER"), "username for PC")
	PublishCommand.Flags().StringVar(&password, "password", os.Getenv("RH_PASSWORD"), "password for PC")
	PublishCommand.Flags().BoolVar(&isOperatorManifest, "is-operator-manifest", false, "if the images sent are operator bundles")

	GetPublishStatusCommand.Flags().StringSliceVar(&urls, "urls", []string{}, "comma separated list of urls")
	GetPublishStatusCommand.Flags().StringVar(&username, "username", os.Getenv("RH_USER"), "username for PC")
	GetPublishStatusCommand.Flags().StringVar(&password, "password", os.Getenv("RH_PASSWORD"), "password for PC")

	GetPublishStatusCommand.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "print logs")
	GetPublishStatusCommand.Flags().Var(
		enumflag.New(&pageSize, "page-size", ListSizeIds, enumflag.EnumCaseInsensitive),
		"page-size",
		"sets logging level; can be")
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

func GetChromeContext(dir string) (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.UserDataDir(dir),
	)

	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)
	return chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
}

type ConnectWebsite struct {
	stop               chan struct{}
	Username, Password string

	directory string
}

var onlyOneSignalHandler = make(chan struct{})
var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

func (c *ConnectWebsite) Start() (context.Context, context.CancelFunc) {
	close(onlyOneSignalHandler) // panics when called twice
	// also set up a custom logger
	if c.directory == "" {
		c.directory = os.TempDir()
	}

	taskCtx, cancel := GetChromeContext(c.directory)

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

	return taskCtx, cancel
}

func (c *ConnectWebsite) Login(inCtx context.Context, username, password string) error {
	var location string
	err := chromedp.Run(inCtx,
		chromedp.Navigate(`https://connect.redhat.com/saml_login`),
		chromedp.Location(&location),
	)

	if err != nil {
		return err
	}

	if location == "https://connect.redhat.com/auth-home" {
		log.Println("already logged in")
		return nil
	}

	err = chromedp.Run(inCtx,
		chromedp.WaitVisible(`input[name="username"]`, chromedp.ByQuery),
		chromedp.Click(`input[name="username"]`, chromedp.ByQuery),
		chromedp.SetValue(`input[name="username"]`, username, chromedp.ByQuery),
		chromedp.Click(`button[data-text-static="Next"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`#step2.show`, chromedp.ByQuery),
		chromedp.Click(`input[name="password"]`, chromedp.ByQuery),
		chromedp.SetValue(`input[name="password"]`, password, chromedp.ByQuery),
		chromedp.Submit(`#kc-login`, chromedp.ByQuery),
		logAction("submit"),
	)

	time.Sleep(5 * time.Second)

	err = chromedp.Run(inCtx,
		chromedp.Location(&location),
	)

	if err != nil {
		return err
	}

	if location == "https://connect.redhat.com/auth-home" {
		log.Println("logged in")
		return nil
	}

	return errors.New("not logged in " + location)
}

type ImagePublishStatus struct {
	Pid           string   `json:"pid"`
	Sha           string   `json:"sha"`
	Tags          []string `json:"tags"`
	PublishStatus string   `json:"publish_status"`
}

func (c *ConnectWebsite) GetAllStatuses(ctx context.Context, url string) (results []*ImagePublishStatus, err error) {
	nodes := []*cdp.Node{}

	log.Println("navigating to", url)

	err = chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady(`table[aria-label="Images List"]`, chromedp.ByQuery),
	)

	if err != nil {
		return
	}

	var pid string

	err = chromedp.Run(ctx,
		chromedp.Click(`button[aria-label="Items per page"]`, chromedp.ByQuery),
		chromedp.Click(fmt.Sprintf(`button[data-action="per-page-%s"]`, pageSize.String()), chromedp.ByQuery),
		chromedp.Nodes(`table[aria-label="Images List"] > tbody`, &nodes, chromedp.ByQueryAll),
		chromedp.Text(`div.rhc-images__header-content__pid-details > small:nth-child(2)`, &pid, chromedp.ByQuery),
	)

	if err != nil {
		return
	}

	for _, node := range nodes {
		var sha, publishStatus string
		localNode := node

		tagNodes := []*cdp.Node{}
		tags := []string{}

		err = chromedp.Run(ctx,
			chromedp.Text(`td[data-label="Image"] > span > span`, &sha, chromedp.ByQuery, chromedp.FromNode(localNode)),
			chromedp.Nodes(`span.rhc-images__list-container__image-tag`, &tagNodes, chromedp.ByQueryAll, chromedp.FromNode(localNode)))

		for _, tagNode := range tagNodes {
			var tag string

			err = chromedp.Run(ctx,
				chromedp.Text("span.pf-c-label__content", &tag, chromedp.ByQuery, chromedp.FromNode(tagNode)),
			)

			if tag != "" {
				tags = append(tags, tag)
			}
		}

		publishStatus, err = PublishStatus(ctx, localNode)

		if err != nil {
			log.Printf("failed to get publish status")
			return
		}

		results = append(results, &ImagePublishStatus{
			Pid:           pid,
			PublishStatus: publishStatus,
			Sha:           sha,
			Tags:          tags,
		})
	}

	log.Printf("found images %v", len(results))

	return
}

func (c *ConnectWebsite) Publish(ctx context.Context, image *imageStruct) (bool, error) {
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

			publishStatus, err := PublishStatus(ctx, localNode)

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

func PublishStatus(ctx context.Context, localNode *cdp.Node) (string, error) {
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
}

func logAction(msg string) chromedp.ActionFunc {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		log.Println(msg)
		return nil
	})
}

func captureScreenshot(name string) chromedp.ActionFunc {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		var buf []byte
		err := chromedp.Run(ctx, chromedp.FullScreenshot(&buf, 90))
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(name, buf, 0644); err != nil {
			log.Fatal(err)
		}
		return nil
	})
}
