// Copyright 2020 IBM Corp.
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

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"emperror.dev/errors"
	pb "github.com/GoogleContainerTools/skaffold/proto"
	env "github.com/caarlos0/env/v6"

	"github.com/gdamore/tcell/v2"
	"github.com/redhat-marketplace/redhat-marketplace-operator/tests/v2/harness"
	"github.com/rivo/tview"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

var logger *log.Logger

type HarnessSkaffoldRun struct {
	Namespace   string `env:"NAMESPACE" envDefault:"openshift-redhat-marketplace"`
	DefaultRepo string `env:"IMAGE_REGISTRY"`

	PullSecretName string `env:"PULL_SECRET_NAME" envDefault:"local-pull-secret"`
	DockerAuthFile string `env:"DOCKER_AUTH_FILE" envDefault:"${HOME}/.docker/config.json" envExpand:"true"`

	FocusSuite string `env:"FOCUS_SUITE"`
}

func main() {
	opts := HarnessSkaffoldRun{}
	env.Parse(&opts)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan)

	process := &tddProcess{
		opts: opts,
	}

	err := process.Run(context.Background())

	if err != nil {
		fmt.Println("failure " + err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}

type tddProcess struct {
	opts HarnessSkaffoldRun

	app          *tview.Application
	skaffoldView *tview.TextView
	ginkgoView   *tview.TextView
	debugView    *tview.TextView
}

func (tdd *tddProcess) Run(inCtx context.Context) error {
	ctx, cancel := context.WithCancel(inCtx)
	skaffoldCmd := &skaffoldCMD{}

	err := skaffoldCmd.Parse()

	if err != nil {
		return err
	}

	ginkgoCmd := &ginkgoTest{}

	err = ginkgoCmd.Parse()
	if err != nil {
		return err
	}

	eventMonitor := &skaffoldEventMonitor{
		ginkgoCmd: ginkgoCmd,
	}

	err = eventMonitor.Parse()
	if err != nil {
		return err
	}

	newPrimitive := func(title string) *tview.TextView {
		return tview.NewTextView().
			SetTextAlign(tview.AlignLeft).
			SetWordWrap(false).
			SetScrollable(true).
			SetDynamicColors(true)
	}

	skaffold := newPrimitive("Skaffold")
	ginkgo := newPrimitive("Ginkgo")
	debug := newPrimitive("Debug")

	logger = log.New(debug, "[tdd-tool] ", log.LstdFlags)

	skaffoldCmd.SetOutput(skaffold)
	ginkgoCmd.SetOutput(tview.ANSIWriter(ginkgo))

	grid := tview.NewGrid().
		SetRows(-1, -3, -1).
		SetBorders(true)

	grid.AddItem(skaffold, 0, 0, 1, 1, 0, 0, false).
		AddItem(ginkgo, 1, 0, 1, 1, 0, 0, false).
		AddItem(debug, 2, 0, 1, 1, 0, 0, false)

	app := tview.NewApplication().
		SetRoot(grid, true).
		SetFocus(grid).
		EnableMouse(true)

	draw := func(view *tview.TextView) func() {
		return func() {
			view.ScrollToEnd()
			app.Draw()
		}
	}
	skaffold.SetChangedFunc(draw(skaffold))
	ginkgo.SetChangedFunc(draw(ginkgo))
	debug.SetChangedFunc(draw(debug))

	skaffoldDone, err := skaffoldCmd.Start(ctx)
	if err != nil {
		return err
	}

	_, err = eventMonitor.Start(ctx)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-skaffoldDone:
				if !ok {
					skaffoldDone, err = skaffoldCmd.Start(ctx)
					return
				}
			}
		}
	}()

	grid.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		logger.Printf("ginkgo key event %v", event)

		if eventMonitor.GetState() == WaitingState {
			eventMonitor.BuildAndDeploy(ctx)
		}

		return event
	})

	if err != nil {
		return err
	}

	logger.Println("starting app")

	err = app.Run()
	cancel()
	<-skaffoldDone

	return err
}

type Process interface {
	Parse() error
	Start(ctx context.Context) (chan interface{}, error)
	Stop()
}

type Run interface {
	Parse() error
	Run(ctx context.Context, args []string) (chan interface{}, error)
}

type skaffoldCMD struct {
	Namespace   string `env:"NAMESPACE" envDefault:"openshift-redhat-marketplace"`
	DefaultRepo string `env:"IMAGE_REGISTRY"`
	RPCPort     string `env:"RPC_PORT" envDefault:"50151"`
	RPCHttpPort string `env:"RPC_HTTP_PORT" envDefault:"50151"`

	cmd *exec.Cmd
	w   io.Writer
}

func (skaffold *skaffoldCMD) Parse() error {
	return env.Parse(skaffold)
}

func (skaffold *skaffoldCMD) SetOutput(w io.Writer) {
	skaffold.w = w
}

func (skaffold *skaffoldCMD) Start(ctx context.Context) (chan error, error) {
	skaffold.cmd = harness.GetCommand(
		"./testbin/skaffold",
		"dev",
		"--port-forward",
		"--trigger=manual",
		"--tail=false",
		"--default-repo",
		skaffold.DefaultRepo,
		"--namespace",
		skaffold.Namespace,
		"--rpc-port=50151",
		"--rpc-http-port=50152",
	)

	skaffold.cmd.Stdout = skaffold.w
	err := skaffold.cmd.Start()

	if err != nil {
		return nil, err
	}

	done := make(chan error, 1)
	go func() {
		select {
		case <-ctx.Done():
			logger.Println("skaffold command killed")
			skaffold.cmd.Process.Signal(syscall.SIGINT)
			return
		case done <- skaffold.cmd.Wait():
			err := <-done

			if err != nil {
				logger.Println("skaffold command errored " + err.Error())
			} else {
				logger.Println("skaffold command complete")
			}
		}
		close(done)
	}()

	return done, nil
}

func (skaffold *skaffoldCMD) Stop() {
	if skaffold.cmd != nil {
		skaffold.cmd.Process.Kill()
	}
}

type ginkgoTest struct {
	DisabledFeatures []string `env:"DISABLED_FEATURES" envSeparator:"," envDefault:"DeployHelm,AddPullSecret"`

	w io.Writer
}

func (ginkgo *ginkgoTest) Parse() error {
	return env.Parse(ginkgo)
}

func (ginkgo *ginkgoTest) SetOutput(w io.Writer) {
	ginkgo.w = w
}

// --progress ./test/integration/ +focus
// focusSuite := "./test/integration"
// if tdd.opts.FocusSuite != "" {
//   focusSuite = "./test/integration/" + tdd.opts.FocusSuite
// }

func (ginkgo *ginkgoTest) Run(ctx context.Context, args ...string) (chan error, error) {
	done := make(chan error)
	cmd := harness.GetCommand("ginkgo", args...)
	cmd.Stdout = ginkgo.w
	cmd.Stdin = &bytes.Buffer{}

	disabled := fmt.Sprintf("%s", strings.Join(ginkgo.DisabledFeatures, ","))
	cmd.Env = append(cmd.Env, fmt.Sprintf("DISABLED_FEATURES=%s", disabled))

	err := cmd.Start()

	if err != nil {
		return nil, err
	}

	go func() {
		select {
		case <-ctx.Done():
			cmd.Process.Signal(os.Interrupt)
			logger.Println("ginkgo command killed")
		case done <- cmd.Wait():
			logger.Println("ginkgo command done")
		}
		close(done)
	}()

	return done, nil
}

type skaffoldEventMonitor struct {
	state     string
	ginkgoCmd *ginkgoTest

	Client pb.SkaffoldServiceClient
	tdd    *tddProcess

	conn           *grpc.ClientConn
	skaffoldEvents chan *pb.LogEntry

	sync.Mutex
}

const (
	BuildState   = "build"
	TestState    = "test"
	WaitingState = "waiting"
)

func (ev *skaffoldEventMonitor) Parse() error {
	return env.Parse(ev)
}

func (ev *skaffoldEventMonitor) dial() error {
	var err error

	for count := 0; count < 5; count = count + 1 {
		ev.conn, err = grpc.Dial("localhost:50151", grpc.WithInsecure())

		if err == nil {
			ev.Client = pb.NewSkaffoldServiceClient(ev.conn)
			_, err := ev.Client.GetState(context.TODO(), &emptypb.Empty{})

			if err == nil {
				return nil
			}

			logger.Println("failed to dial " + err.Error())
			time.Sleep(1 * time.Second)
			continue
		}

		time.Sleep(1 * time.Second)
	}

	if err != nil {
		logger.Println("failed to dial " + err.Error())
		return errors.Wrap(err, "error dialing")
	}

	return nil
}

func (ev *skaffoldEventMonitor) DisableAutobuild(ctx context.Context) {

	ev.Client.AutoBuild(ctx, &pb.TriggerRequest{
		State: &pb.TriggerState{
			Val: &pb.TriggerState_Enabled{
				Enabled: false,
			},
		},
	})
	ev.Client.AutoDeploy(ctx, &pb.TriggerRequest{
		State: &pb.TriggerState{
			Val: &pb.TriggerState_Enabled{
				Enabled: false,
			},
		},
	})

}

func (ev *skaffoldEventMonitor) Start(ctx context.Context) (chan *pb.LogEntry, error) {
	err := ev.dial()

	if err != nil {
		return nil, err
	}

	skaffoldEvents := make(chan *pb.LogEntry)

	go func() {
		defer close(skaffoldEvents)
		defer ev.conn.Close()

		eventsClient, err := ev.Client.Events(ctx, &emptypb.Empty{})

		if err != nil {
			log.Print("failed to get client", err)
			return
		}

		for {
			in, err := eventsClient.Recv()
			skaffoldEvents <- in
			if err == io.EOF {
				logger.Println("skaffold events pipe was closed")
				return
			}
			select {
			case <-ctx.Done():
				logger.Println("shutting down events")
				return
			default:
			}
		}
	}()

	go func() {
		for in := range skaffoldEvents {
			ev.handleEvents(ctx, in)
		}
	}()

	return skaffoldEvents, nil
}

func (ev *skaffoldEventMonitor) BuildAndDeploy(ctx context.Context) error {
	in := pb.UserIntentRequest{
		Intent: &pb.Intent{
			Build:  true,
			Deploy: true,
			Sync:   true,
		},
	}
	_, err := ev.Client.Execute(ctx, &in)

	if err != nil {
		return err
	}

	return nil
}

func (ev *skaffoldEventMonitor) setState(state string) {
	ev.state = state
}

func (ev *skaffoldEventMonitor) RunTest(ctx context.Context) error {
	logger.Println("starting test")
	args := []string{"--progress", "-r", "./test/integration/"}
	done, err := ev.ginkgoCmd.Run(ctx, args...)

	if err != nil {
		return err
	}

	go func() {
		<-done
		ev.Lock()
		defer ev.Unlock()
		ev.state = WaitingState
	}()

	return nil
}

func (ev *skaffoldEventMonitor) GetState() string {
	ev.Lock()
	defer ev.Unlock()
	return ev.state
}

func (ev *skaffoldEventMonitor) handleEvents(ctx context.Context, in *pb.LogEntry) {
	ev.Lock()
	defer ev.Unlock()

	if in == nil || in.Event == nil {
		return
	}

	logger.Printf("received event %v", in)

	if evt := in.Event.GetDevLoopEvent(); evt != nil {
		if evt.GetStatus() == "Succeeded" {
			logger.Println("received succeed status, starting test")
			ev.setState(TestState)
			ev.DisableAutobuild(ctx)
			ev.RunTest(ctx)
		} else if evt.GetStatus() == "Failed" {
			logger.Println("received failed status, press a button to retry")
			ev.setState(WaitingState)
		} else {
			logger.Println("received build status " + evt.GetStatus())
			ev.setState(BuildState)
		}
	}
}
