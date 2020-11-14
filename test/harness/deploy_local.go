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

package harness

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/caarlos0/env"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type deployLocal struct {
	PullSecretName string `env:"PULL_SECRET_NAME" envDefault:"local-pull-secret"`
	Namespace      string `env:"NAMESPACE" envDefault:"openshift-redhat-marketplace"`

	cleanup []runtime.Object
}

func (d *deployLocal) Name() string {
	return "DeployLocal"
}

func (d *deployLocal) Parse() error {
	return env.Parse(d)
}

func (d *deployLocal) HasCleanup() []runtime.Object {
	return d.cleanup
}

func (d *deployLocal) Setup(h *TestHarness) error {
	d.cleanup = []runtime.Object{}
	command := exec.Command("make", "clean", "helm")
	command.Dir = "../.."
	command.Env = os.Environ()
	err := command.Run()
	out, _ := command.CombinedOutput()

	if err != nil {
		fmt.Println(string(out))
		return errors.Wrap(err, "fail")
	}

	operatorDepl := &appsv1.Deployment{}
	dat, err := ioutil.ReadFile("../../deploy/operator.yaml")

	if err != nil {
		return errors.Wrap(err, "fail")
	}

	err = yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(dat)), 100).Decode(operatorDepl)

	if err != nil {
		return errors.Wrap(err, "fail")
	}

	for _, env := range operatorDepl.Spec.Template.Spec.Containers[0].Env {
		os.Setenv(env.Name, env.Value)
	}

	type runtimeFile struct {
		filename string
		findType func(string) runtime.Object
	}

	loadFiles := []runtimeFile{
		{"../../deploy/service_account.yaml", func(dat string) runtime.Object {
			sa := &corev1.ServiceAccount{}
			sa.Namespace = d.Namespace
			sa.ImagePullSecrets = append(sa.ImagePullSecrets, corev1.LocalObjectReference{
				Name: d.PullSecretName,
			})
			return sa
		}},
		{"../../deploy/role.yaml", func(dat string) runtime.Object {
			switch {
			case strings.Contains(dat, "kind: Role"):
				GinkgoWriter.Write([]byte("adding kind Role\n"))
				obj := &rbacv1.Role{}
				return obj
			case strings.Contains(dat, "kind: ClusterRole"):
				GinkgoWriter.Write([]byte("adding kind ClusterRole\n"))
				obj := &rbacv1.ClusterRole{}
				return obj
			default:
				GinkgoWriter.Write([]byte("type not found\n"))
				return nil
			}
		}},
		{"../../deploy/role_binding.yaml", func(dat string) runtime.Object {
			switch {
			case strings.Contains(dat, "kind: RoleBinding"):
				obj := &rbacv1.RoleBinding{}
				GinkgoWriter.Write([]byte("adding kind RoleBinding\n"))
				return obj
			case strings.Contains(dat, "kind: ClusterRoleBinding"):
				obj := &rbacv1.ClusterRoleBinding{}
				GinkgoWriter.Write([]byte("adding kind ClusterRoleBinding\n"))
				return obj
			default:
				GinkgoWriter.Write([]byte("type not found\n"))
				return nil
			}
		}},
	}

	for _, rec := range loadFiles {
		dat, err := ioutil.ReadFile(rec.filename)
		Expect(err).ShouldNot(HaveOccurred())

		datArray := strings.Split(string(dat), "---")
		for _, dat := range datArray {
			obj := rec.findType(dat)

			if obj == nil || len(dat) == 0 {
				continue
			}

			err := yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(dat)), 100).Decode(obj)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(h.Upsert(context.TODO(), obj)).To(Succeed(), rec.filename, " ", obj)
			d.cleanup = append(d.cleanup, obj)
		}
	}

	return nil
}
