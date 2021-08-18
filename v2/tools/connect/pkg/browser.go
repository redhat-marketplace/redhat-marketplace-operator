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
	"regexp"
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
	dryRun             bool
	verbose            bool

	pageSize ListSize = ListSize50
)

var GetPublishStatusCommand = &cobra.Command{
	Use:   "status",
	Short: "Retrieves the current status of publish",
	PreRun: func(cmd *cobra.Command, args []string) {
		if !verbose {
			log.SetOutput(ioutil.Discard)
		} else {
			log.SetOutput(cmd.OutOrStderr())
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		urlToShas, err := parseArgs()

		if err != nil {
			return err
		}

		statuses := []*ImagePublishStatus{}

		err = cmdFunction(cmd, args, urlToShas, func(_ context.Context, url string, _ *cdp.Node, img *ImagePublishStatus) error {
			matched := matchAndMark(urlToShas, img)

			if !matched {
				return nil
			}

			statuses = append(statuses, img)
			return nil
		})

		if err != nil {
			return err
		}

		data, err := json.Marshal(statuses)

		if err != nil {
			return err
		}

		cmd.OutOrStdout().Write(data)
		return nil
	},
}

const (
	Passed = "Passed"
	Failed = "Failed"

	Published = "Published"
)

var PublishCommand = &cobra.Command{
	Use:   "publish",
	Short: "Publish's the image on PC.",
	PreRun: func(cmd *cobra.Command, args []string) {
		if !verbose {
			log.SetOutput(ioutil.Discard)
		} else {
			log.SetOutput(cmd.OutOrStderr())
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		urlToShas, err := parseArgs()

		if err != nil {
			return err
		}

		err = cmdFunction(cmd, args, urlToShas,
			func(ctx context.Context, url string, node *cdp.Node, result *ImagePublishStatus) error {
				matched := matchAndMark(urlToShas, result)

				if !matched {
					return nil
				}

				if result.CertificationStatus == Passed && result.PublishStatus != Published {
					if dryRun {
						return nil
					}

					err := publishImageByNode(ctx, node)

					if err != nil {
						log.Println("failed to publish", err)
					}

					log.Println("publishing result", result)
				} else if result.CertificationStatus == Passed && result.PublishStatus == Published {
					log.Println("image is already published", result)
				} else {
					log.Println("image is not ready to publish", result)
				}

				return nil
			})

		if err != nil {
			log.Println("failed to publish", err)
			return err
		}

		return nil
	},
}

func parseArgs() (map[string]map[imageStruct]bool, error) {
	csvStr := strings.Join(append([]string{"project_url,sha,tag"}, images...), "\n")
	csvRows, err := csv.NewReader(strings.NewReader(csvStr)).ReadAll()

	if err != nil {
		return nil, err
	}

	urlToShas := make(map[string]map[imageStruct]bool)

	for _, row := range csvRows[1:] {
		image := &imageStruct{}
		image.FromCSV(row)

		if _, ok := urlToShas[image.ProjectURL]; !ok {
			urlToShas[image.ProjectURL] = make(map[imageStruct]bool)
		}

		urlToShas[image.ProjectURL][*image] = false
	}

	return urlToShas, nil
}

func cmdFunction(
	cmd *cobra.Command,
	args []string,
	urlToShas map[string]map[imageStruct]bool,
	work func(context.Context, string, *cdp.Node, *ImagePublishStatus) error,
) error {
	web := &ConnectWebsite{
		Username: username,
		Password: password,
	}

	webCtx, webCancel := web.Start()
	defer webCancel()

	overallCtx, cancel := context.WithTimeout(webCtx, 8*time.Minute)
	defer cancel()

	err := func() error {
		return web.Login(overallCtx, username, password)
	}()

	if err != nil {
		log.Println("failed to login", err)
		return err
	}

	for url, shas := range urlToShas {
		log.Println("looking for", url, shas)
		ctx, lcancel := context.WithTimeout(overallCtx, 4*time.Minute)
		defer lcancel()

		err := web.walkImages(ctx, url, false, work)

		if errors.Is(err, Stop) {
			continue
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func allFoundForUrl(urlToShas map[string]map[imageStruct]bool, url string) bool {
	foundAll := true

	for _, k := range urlToShas[url] {
		foundAll = foundAll && k
	}

	return foundAll
}

func matchAndMark(urlToShas map[string]map[imageStruct]bool, img *ImagePublishStatus) bool {
	matched := false
	shas := urlToShas[img.Url]

	for k := range shas {
		if match(&k, img) {
			shas[k] = true
			matched = true
		}
	}

	if !matched {
		return false
	}

	return true
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

func BindCmd(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&username, "username", "", "username for PC")
	cmd.PersistentFlags().StringVar(&password, "password", "", "password for PC")
	cmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "password for PC")
	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "print logs")

	PublishCommand.Flags().StringArrayVar(&images, "images", []string{}, "list comma seperated image vars to publish")
	GetPublishStatusCommand.Flags().StringArrayVar(&images, "images", []string{}, "list comma seperated image vars to publish")

	GetPublishStatusCommand.Flags().Var(
		enumflag.New(&pageSize, "page-size", ListSizeIds, enumflag.EnumCaseInsensitive),
		"page-size",
		"sets logging level; can be")

	cmd.AddCommand(WaitAndPublishCmd)
	cmd.AddCommand(PublishCommand)
	cmd.AddCommand(GetPublishStatusCommand)
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
	if username == "" || password == "" {
		return errors.New("no username or password passed")
	}

	var location string
	err := chromedp.Run(inCtx,
		chromedp.Navigate(`https://connect.redhat.com/saml_login`),
		chromedp.Location(&location),
	)

	if err != nil {
		return err
	}

	log.Println("location is", location)

	if location == "https://connect.redhat.com/auth-home" {
		log.Println("already logged in")
		return nil
	}

	err = chromedp.Run(inCtx,
		chromedp.WaitVisible(`input[name="username"]`, chromedp.ByQuery),
		chromedp.Click(`input[name="username"]`, chromedp.ByQuery),
		logAction("username input"),
		chromedp.SetValue(`input[name="username"]`, username, chromedp.ByQuery),
		chromedp.Click(`button[data-text-static="Next"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`#step2.show`, chromedp.ByQuery),
		chromedp.Click(`input[name="password"]`, chromedp.ByQuery),
		chromedp.SetValue(`input[name="password"]`, password, chromedp.ByQuery),
		logAction("password input"),
		chromedp.Submit(`#kc-login`, chromedp.ByQuery),
		logAction("submit"),
	)

	timeoutCtx, cancel := context.WithTimeout(inCtx, 20*time.Second)
	defer cancel()

	for {
		err = chromedp.Run(inCtx,
			chromedp.Location(&location),
		)

		if err != nil {
			return err
		}

		log.Println("location is", location)

		if location == "https://connect.redhat.com/auth-home" {
			log.Println("logged in")
			return nil
		}

		select {
		case <-timeoutCtx.Done():
			return errors.New("timed out")
		default:
			time.Sleep(2 * time.Second)
		}

	}
}

type ImagePublishStatus struct {
	Pid                 string   `json:"pid"`
	Name                string   `json:"name"`
	Url                 string   `json:"url"`
	Sha                 string   `json:"sha"`
	Tags                []string `json:"tags"`
	PublishStatus       string   `json:"publish_status"`
	CertificationStatus string   `json:"certification_status"`
}

func match(inImage *imageStruct, pageImage *ImagePublishStatus) bool {
	if inImage.ProjectURL != pageImage.Url {
		return false
	}

	if inImage.SHA != "" && pageImage.Sha == inImage.SHA {
		return true
	}

	if inImage.SHA == "" && inImage.Tag != "" {
		for _, tag := range pageImage.Tags {
			if matched, _ := regexp.Match(inImage.Tag, []byte(tag)); matched {
				return true
			}
		}
	}

	return false
}

var (
	Stop error = errors.New("stop")
)

func (c *ConnectWebsite) walkImages(
	ctx context.Context, url string, walkAllPages bool,
	imageAction func(context.Context, string, *cdp.Node, *ImagePublishStatus) error,
) error {
	nodes := []*cdp.Node{}
	var location string

	err := chromedp.Run(ctx,
		chromedp.Location(&location),
	)

	if err != nil {
		return err
	}

	if url != location {
		log.Println("navigating to", url)

		err = chromedp.Run(ctx, chromedp.Navigate(url))

		if err != nil {
			return err
		}
	}

	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`table[aria-label="Images List"]`, chromedp.ByQuery),
	)

	if err != nil {
		return err
	}

	log.Println("ready")

	var pid, name string

	err = chromedp.Run(ctx,
		chromedp.Click(`button[aria-label="Items per page"]`, chromedp.ByQuery),
		chromedp.WaitVisible(fmt.Sprintf(`button[data-action="per-page-%s"]`, pageSize.String()), chromedp.ByQuery),
		chromedp.Click(fmt.Sprintf(`button[data-action="per-page-%s"]`, pageSize.String()), chromedp.ByQuery),
	)
	if err != nil {
		log.Println(err)
		return err
	}

	for {
		err = chromedp.Run(ctx,
			chromedp.Nodes(`table[aria-label="Images List"] > tbody`, &nodes, chromedp.ByQueryAll),
			chromedp.Text(`.rhc-images__header-content__pid-details > small:nth-child(2)`, &pid, chromedp.ByQuery),
			chromedp.Text(`.project-details-header__primary__info--name`, &name, chromedp.ByQuery),
		)

		if err != nil {
			log.Println(err)
			return err
		}

		for _, node := range nodes {
			var sha, pubStatus, certStatus string
			var tags []string

			localNode := node
			tags, err = getTags(ctx, localNode)

			if err != nil {
				log.Printf("failed to get tags")
				return err
			}

			err = chromedp.Run(ctx,
				chromedp.Text(`td[data-label="Image"] > span > span`, &sha, chromedp.ByQuery, chromedp.FromNode(localNode)),
				chromedp.Text(`td[data-label="Certification test"]`, &certStatus, chromedp.ByQuery, chromedp.FromNode(localNode)))

			if err != nil {
				log.Printf("failed to get sha and cert status")
				return err
			}

			pubStatus, err = publishStatus(ctx, localNode)

			if err != nil {
				log.Printf("failed to get publish status")
				return err
			}

			img := &ImagePublishStatus{
				Pid:                 pid,
				Name:                name,
				Url:                 url,
				PublishStatus:       pubStatus,
				Sha:                 sha,
				Tags:                tags,
				CertificationStatus: certStatus,
			}

			err := imageAction(ctx, url, localNode, img)

			if errors.Is(err, Stop) {
				return nil
			}

			if err != nil {
				return err
			}
		}

		if !walkAllPages {
			return nil
		}

		log.Println("going to next page")
		// look for next page
		err = c.nextPage(ctx)
		if err != nil {
			if !errors.Is(err, ErrNoPages) {
				return err
			}

			break
		}
	}

	return nil
}

const nextButtonNotDisabled = `button[data-action="next"]:not(.pf-m-disabled)`

var ErrNoPages = errors.New("no more pages")

func (c *ConnectWebsite) nextPage(ctx context.Context) error {
	quickCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var pageDetails string

	nodes := []*cdp.Node{}

	err := chromedp.Run(quickCtx,
		chromedp.Nodes(nextButtonNotDisabled, &nodes, chromedp.ByQuery),
	)

	if len(nodes) == 0 {
		log.Println("no more pages")
		return ErrNoPages
	}

	err = chromedp.Run(quickCtx,
		chromedp.Click(nextButtonNotDisabled, chromedp.ByQuery),
		chromedp.TextContent(`span.pf-c-options-menu__toggle-text`, &pageDetails, chromedp.ByQuery),
	)

	if err != nil {
		return errors.New("no more pages")
	}

	log.Println("next page", pageDetails)
	return nil
}

func (c *ConnectWebsite) Publish(ctx context.Context, image *ImagePublishStatus) (bool, error) {
	url := image.Url
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

	for {
		found := false

		for _, node := range nodes {
			var sha, tag string

			localNode := node
			err = chromedp.Run(ctx,
				chromedp.Text(`td[data-label="Image"] > span > span`, &sha, chromedp.ByQuery, chromedp.FromNode(localNode)),
			)

			if err != nil {
				return false, err
			}

			if sha == image.Sha {
				found = true
				publishStatus, err := publishStatus(ctx, localNode)

				if err != nil {
					log.Printf("failed to get publish status")
					return false, err
				}

				if publishStatus == Published {
					log.Println("sha is published", sha)
					return true, nil
				}

				if dryRun {
					log.Println("dry run", sha)
					return true, nil
				}

				err = publishImageByNode(ctx, localNode)

				if err != nil {
					return false, err
				}

				log.Println("publishing finished", sha, tag)
				return true, nil
			}
		}

		// look for next page
		if !found {
			err = c.nextPage(ctx)
			if err != nil {
				if !errors.Is(err, ErrNoPages) {
					return false, err
				}

				break
			}
		}
	}

	return false, nil
}

func publishImageByNode(ctx context.Context, localNode *cdp.Node) error {
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
}

func containsTag(tags []string, matches func(tag string) bool) bool {
	for _, tag := range tags {
		if matches(tag) {
			return true
		}
	}

	return false
}

func getTags(ctx context.Context, localNode *cdp.Node) ([]string, error) {
	tagNodes := []*cdp.Node{}
	err := chromedp.Run(ctx,
		chromedp.Nodes(`span.rhc-images__list-container__image-tag`, &tagNodes, chromedp.ByQueryAll, chromedp.FromNode(localNode)))

	if err != nil {
		return nil, err
	}

	tags := []string{}
	for _, tagNode := range tagNodes {
		var tag string

		err = chromedp.Run(ctx,
			chromedp.Text("span.pf-c-label__content", &tag, chromedp.ByQuery, chromedp.FromNode(tagNode)),
		)

		if tag != "" {
			tags = append(tags, tag)
		}
	}

	return tags, nil
}

func publishStatus(ctx context.Context, localNode *cdp.Node) (string, error) {
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
