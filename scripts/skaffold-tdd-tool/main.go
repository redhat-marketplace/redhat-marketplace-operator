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
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"

	"time"

	pb "github.com/GoogleContainerTools/skaffold/proto"
	env "github.com/caarlos0/env/v6"
	"github.com/fatih/color"
	"github.com/redhat-marketplace/redhat-marketplace-operator/test/harness"
	"github.com/redhat-marketplace/redhat-marketplace-operator/test/testenv"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type HarnessSkaffoldRun struct {
	Namespace   string `env:"NAMESPACE" envDefault:"openshift-redhat-marketplace"`
	DefaultRepo string `env:"IMAGE_REGISTRY"`

	PullSecretName string `env:"PULL_SECRET_NAME" envDefault:"local-pull-secret"`
	DockerAuthFile string `env:"DOCKER_AUTH_FILE" envDefault:"${HOME}/.docker/config.json" envExpand:"true"`
}

func errr(err error) {
	fmt.Printf("failed to start skaffold test run %s\n", err.Error())
}

func main() {
	opts := HarnessSkaffoldRun{}
	env.Parse(&opts)

	testHarness, _ := harness.NewTestHarness(harness.TestHarnessOptions{
		EnabledFeatures: []string{
			harness.FeatureAddPullSecret,
			harness.FeatureMockOpenShift,
		},
		Namespace:       "openshift-redhat-marketplace",
		WatchNamespace:  "",
		ProvideScheme:   testenv.InitializeScheme,
	})

	testHarness.Start()

	ctx, cancel := context.WithCancel(context.Background())

	cmd := harness.GetCommand("./testbin/skaffold", "dev",
		"--port-forward",
		"--trigger", "manual",
		"--tail=false",
		"--default-repo", opts.DefaultRepo,
		"--namespace", opts.Namespace,
		"--rpc-port=50151", "--rpc-http-port=50152",
	)

	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin

	err := cmd.Start()

	if err != nil {
		errr(err)
		os.Exit(1)
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
		close(waitCh)
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan)

	triggerTest := make(chan bool, 1)
	go func() {
		defer close(triggerTest)
		var waitCh chan error
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-triggerTest:
				waitCh = make(chan error, 1)
				if !ok {
					return
				}

				cmd := harness.GetCommand("ginkgo", "-r", "--randomizeAllSpecs", "--randomizeSuites", "--progress", "--trace", "./test/integration")
				disabled := os.Getenv("DISABLED_FEATURES")

				if disabled != "" {
					disabled = fmt.Sprintf(",%s", disabled)
				}

				cmd.Env = append(cmd.Env, fmt.Sprintf("DISABLED_FEATURES=DeployHelm,AddPullSecret%s", disabled))
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout

				cmd.Start()

				go func() {
					waitCh <- cmd.Wait()
					close(waitCh)
				}()

				go func() {
					select {
					case <-ctx.Done():
						sigChan <- os.Interrupt
					case sig := <-sigChan:
						if err := cmd.Process.Signal(sig); err != nil {
							log.Print("error sending signal", sig, err)
						}
						return
					}
				}()

				select {
				case <-ctx.Done():
					return
				case <-waitCh:
				}
			}
		}
	}()

	eventsChan := skaffoldEventMonitor(ctx, triggerTest, os.Stdout)

	// You need a for loop to handle multiple signals
process:
	for {
		select {
		case err := <-eventsChan:
			log.Print("error receiving events", err)
			sigChan <- os.Interrupt
			cancel()
		case sig := <-sigChan:
			if err := cmd.Process.Signal(sig); err != nil {
				log.Print("error sending signal", sig, err)
			}
		case err := <-waitCh:
			// Subprocess exited. Get the return code, if we can
			var waitStatus syscall.WaitStatus
			if exitError, ok := err.(*exec.ExitError); ok {
				waitStatus = exitError.Sys().(syscall.WaitStatus)
				cancel()

				testHarness.Stop()
				os.Exit(waitStatus.ExitStatus())
			}
			if err != nil {
				log.Fatal(err)
			}

			cancel()
			break process
		}
	}

	testHarness.Stop()
	os.Exit(0)
}

func skaffoldEventMonitor(ctx context.Context, triggerTest chan bool, w io.Writer) chan error {
	eventsChan := make(chan error, 1)

	go func() {
		defer close(eventsChan)
		time.Sleep(5 * time.Second)
		conn, err := grpc.Dial("localhost:50151", grpc.WithInsecure())
		if err != nil {
			log.Fatalf("fail to dial: %v", err)
		}
		defer conn.Close()

		client := pb.NewSkaffoldServiceClient(conn)
		eventsClient, err := client.Events(context.Background(), &emptypb.Empty{})

		if err != nil {
			log.Print("failed to get client", err)
			eventsChan <- err
			return
		}

		yellow := color.New(color.FgYellow).SprintFunc()
		fmt.Fprintf(w, "\n%s - starting skaffold event monitor\n", yellow("[tdd-skaffold]"))

		skaffoldEvents := make(chan *pb.LogEntry)
		go func() {
			for {
				in, err := eventsClient.Recv()

				if err != nil {
					if err != io.EOF {
						break
					}

					fmt.Fprintf(w, "\n%s - error %s\n", yellow("[tdd-skaffold]"), err.Error())
				}

				skaffoldEvents <- in
			}

			close(skaffoldEvents)
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case in := <-skaffoldEvents:
				if in == nil {
					continue
				}

				if evt := in.Event.GetBuildEvent(); evt != nil {
					fmt.Fprintf(w, "\n%s - build %s\n", yellow("[tdd-skaffold]"), evt.GetStatus())
				} else if deployEvt := in.Event.GetDeployEvent(); deployEvt != nil {
					fmt.Fprintf(w, "\n%s - deploy %s\n", yellow("[tdd-skaffold]"), deployEvt.GetStatus())

					if deployEvt.GetStatus() == "Completed" {
						triggerTest <- true
					}
				} else if evt := in.Event.GetDevLoopEvent(); evt != nil {
					fmt.Fprintf(w, "\n%s - dev loop %s\n", yellow("[tdd-skaffold]"), evt.GetStatus())

					if evt.GetStatus() == "Succeeded" {
						triggerTest <- true
					}
				}  else {
					fmt.Fprintf(w, "\n%s - received event %s\n", yellow("[tdd-skaffold]"), in.Event.GetEventType())
				}
			}
		}
	}()

	return eventsChan
}

type SyncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (sw *SyncWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.w.Write(p)
}

func kubeCreate(opts HarnessSkaffoldRun) error {

	// pullSecretName, dockerAuthFile, namespace := opts.PullSecretName, opts.DockerAuthFile, opts.Namespace
	// home := homedir.HomeDir()
	// kubeconfig := filepath.Join(home, ".kube", "config")

	// config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	// if err != nil {
	// 	panic(err)
	// }
	// clientset, err := kubernetes.NewForConfig(config)
	// if err != nil {
	// 	panic(err)
	// }

	// data, err := ioutil.ReadFile(dockerAuthFile)
	// if err != nil {
	// 	return errors.Wrap(err, "failed to read docker auth file")
	// }

	// os.Setenv("PULL_SECRET_NAME", pullSecretName)

	// pullSecret := v1.Secret{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name:      pullSecretName,
	// 		Namespace: namespace,
	// 	},
	// 	Type: v1.SecretTypeDockerConfigJson,
	// 	Data: map[string][]byte{
	// 		v1.DockerConfigJsonKey: data,
	// 	},
	// }
	// namespaceClient := clientset.CoreV1().Namespaces()

	// additionalNamespaces := []string{
	// 	namespace,
	// 	"openshift-monitoring",
	// 	"openshift-config-managed",
	// 	"openshift-config",
	// }

	// for _, ns := range additionalNamespaces {
	// 	namespaceClient.Create(context.TODO(), &v1.Namespace{
	// 		ObjectMeta: metav1.ObjectMeta{
	// 			Name: ns,
	// 		},
	// 	}, metav1.CreateOptions{})
	// }

	// secretClient := clientset.CoreV1().Secrets(namespace)

	// secretClient.Delete(context.TODO(), pullSecretName, metav1.DeleteOptions{})
	// secretClient.Create(context.TODO(), &pullSecret, metav1.CreateOptions{})

	return nil
}
