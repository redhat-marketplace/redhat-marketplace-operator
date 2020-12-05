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
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/util/retry"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// This is for mock controllers to run in the background for kuttl

var kubeconfig, namespace *string
var client dynamic.Interface
var config *rest.Config
var serverKeyFile, serverCertFile, caCertFile *string

var log = logf.Log.WithName("mockcontroller")

func main() {
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = pflag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = pflag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	namespace = pflag.String("namespace", "openshift-redhat-marketplace", "namespace to target")
	serverKeyFile = pflag.String("server-key", "", "Server secret")
	serverCertFile = pflag.String("server-cert", "", "Server cert")
	caCertFile = pflag.String("ca-cert", "", "ca cert")
	pflag.Parse()

	var err error
	config, err = rest.InClusterConfig()

	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			panic(err)
		}

		panic(err.Error())
	}

	client, err = dynamic.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	termChan := make(chan os.Signal)
	signal.Notify(termChan, syscall.SIGINT, syscall.SIGTERM)
	log.Info("starting")
	ctx, cancel := context.WithCancel(context.Background())
	go handleSecretGeneration(ctx)
	go handleCABundleInsertion(ctx)
	t := <-termChan
	log.Error(err, "getting killed", "signal", t)
	cancel()
	log.Info("exiting")
}

func handleCABundleInsertion(ctx context.Context) {
	configRes := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	configmapWatcher, err := client.Resource(configRes).
		Namespace(*namespace).
		Watch(ctx, metav1.ListOptions{})

	if err != nil {
		log.Error(err, "failed to watch")
		panic(err)
	}

	serviceCA, err := ioutil.ReadFile(*caCertFile)
	if err != nil {
		log.Error(err, "failed to read caCert")
		panic(err)
	}

	defer configmapWatcher.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("done called")
			return
		case result := <-configmapWatcher.ResultChan():
			switch result.Type {
			case watch.Added:
				fallthrough
			case watch.Bookmark:
				fallthrough
			case watch.Modified:
				obj, _ := meta.Accessor(result.Object)
				annotations := obj.GetAnnotations()
				if v, ok := annotations["service.beta.openshift.io/inject-cabundle"]; ok && v == "true" {
					getResult, _ := client.Resource(configRes).Namespace(*namespace).Get(ctx, obj.GetName(), metav1.GetOptions{})

					if v, ok := getResult.Object["data"]; ok {
						if v == nil {
							continue
						}
						if _, ok := v.(map[string]interface{}); !ok {
							continue
						}

						if crt, ok := v.(map[string]interface{})["service-ca.crt"]; ok && crt.(string) == string(serviceCA) {
							continue
						}
					}

					err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
						getResult.Object["data"] = map[string]string{
							"service-ca.crt": string(serviceCA),
						}

						_, err := client.Resource(configRes).Namespace(*namespace).Update(ctx, getResult, metav1.UpdateOptions{})

						return err
					})

					if err != nil {
						log.Error(err, "failed to update serviceCA")
					}
				}
			}
		}
	}
}

func handleSecretGeneration(ctx context.Context) {
	secretRes := schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
	servicesRes := schema.GroupVersionResource{Version: "v1", Resource: "services"}
	serviceWatcher, err := client.Resource(servicesRes).
		Namespace(*namespace).
		Watch(ctx, metav1.ListOptions{})

	if err != nil {
		log.Error(err, "failed to watch services")
		panic(err)
	}

	serverCrt, err := ioutil.ReadFile(*serverCertFile)
	if err != nil {
		log.Error(err, "failed to read serverCert")
		panic(err)
	}

	serverKey, err := ioutil.ReadFile(*serverKeyFile)
	if err != nil {
		log.Error(err, "failed to read serverKey")
		panic(err)
	}

	defer serviceWatcher.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("exiting because we're done")
			return
		case result := <-serviceWatcher.ResultChan():
			switch result.Type {
			case watch.Added:
				obj, _ := meta.Accessor(result.Object)

				annotations := obj.GetAnnotations()

				if secretName, ok := annotations["service.beta.openshift.io/serving-cert-secret-name"]; ok {
					secret := &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Secret",
							"metadata": map[string]interface{}{
								"name":      secretName,
								"namespace": *namespace,
							},
							"type": "tls",
							"data": map[string][]byte{
								"tls.crt": serverCrt,
								"tls.key": serverKey,
							},
						},
					}

					_, err := client.Resource(secretRes).Namespace(*namespace).Get(ctx, secret.GetName(), metav1.GetOptions{})

					if errors.IsNotFound(err) {
						err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
							_, err := client.Resource(secretRes).Namespace(*namespace).Create(ctx, secret, metav1.CreateOptions{})

							return err
						})

						if err != nil {
							log.Error(err, "failed to create resource")
						}
					}
				}
			}
		}
	}
}
