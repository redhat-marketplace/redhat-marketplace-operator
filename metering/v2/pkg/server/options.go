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

package server

import (
	"flag"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/types"
	"github.com/spf13/pflag"
	"k8s.io/kube-state-metrics/pkg/options"
)

// Options are the configurable parameters
type Options struct {
	Apiserver     string
	Kubeconfig    string
	Help          bool
	Port          int
	Host          string
	TelemetryPort int
	TelemetryHost string
	Namespaces    options.NamespaceList
	Pod           string
	Namespace     string
	Version       bool

	EnableGZIPEncoding bool

	flags *pflag.FlagSet
}

func ProvideNamespaces(opts *Options) types.Namespaces {
	if opts.Namespaces == nil || len(opts.Namespaces) == 0 {
		opts.Namespaces = DefaultNamespaces
	}

	return types.Namespaces(opts.Namespaces)
}

// NewOptions returns a new instance of `Options`.
func NewOptions() *Options {
	return &Options{}
}

const trueStr = "true"

func (o *Options) AddFlags() {
	o.flags = pflag.NewFlagSet("", pflag.ExitOnError)
	// add klog flags
	klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(klogFlags)
	o.flags.AddGoFlagSet(klogFlags)
	if f := o.flags.Lookup("logtostderr"); f != nil {
		//nolint:errcheck
		f.Value.Set(trueStr)
	}
	o.flags.Lookup("logtostderr").DefValue = trueStr
	o.flags.Lookup("logtostderr").NoOptDefVal = trueStr

	o.flags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		o.flags.PrintDefaults()
	}

	o.flags.StringVar(&o.Apiserver, "apiserver", "", `The URL of the apiserver to use as a master`)
	o.flags.StringVar(&o.Kubeconfig, "kubeconfig", "", "Absolute path to the kubeconfig file")
	o.flags.BoolVarP(&o.Help, "help", "h", false, "Print Help text")
	o.flags.IntVar(&o.Port, "port", 8080, `Port to expose metrics on.`)
	o.flags.StringVar(&o.Host, "host", "0.0.0.0", `Host to expose metrics on.`)
	o.flags.IntVar(&o.TelemetryPort, "telemetry-port", 8081, `Port to expose kube-state-metrics self metrics on.`)
	o.flags.StringVar(&o.TelemetryHost, "telemetry-host", "0.0.0.0", `Host to expose kube-state-metrics self metrics on.`)
	o.flags.Var(&o.Namespaces, "namespaces", fmt.Sprintf("Comma-separated list of namespaces to be enabled. Defaults to %q", &DefaultNamespaces))

	o.flags.StringVar(&o.Pod, "pod", "", "Name of the pod that contains the kube-state-metrics container.")
	o.flags.StringVar(&o.Namespace, "pod-namespace", "", "Name of the namespace of the pod specified by --pod.")
	o.flags.BoolVarP(&o.Version, "version", "", false, "kube-state-metrics build version information")
	o.flags.BoolVar(&o.EnableGZIPEncoding, "enable-gzip-encoding", false, "Gzip responses when requested by clients via 'Accept-Encoding: gzip' header.")
}

func (o *Options) Mount(addFlags func(newSet *pflag.FlagSet)) {
	o.AddFlags()
	addFlags(o.flags)
}

var DefaultNamespaces = options.NamespaceList{metav1.NamespaceAll}
