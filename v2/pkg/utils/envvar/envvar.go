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

package envvar

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
)

type Changes []envVarAction

func (e *Changes) Append(changes Changes) {
	*e = append(*e, changes...)
}

func (e *Changes) Add(env corev1.EnvVar) {
	*e = append(*e, Add(env))
}

func (e *Changes) Remove(env corev1.EnvVar) {
	*e = append(*e, Remove(env))
}

func Add(env corev1.EnvVar) envVarAction {
	return envVarAction{
		Add: true,
		Var: env,
	}
}

func Remove(env corev1.EnvVar) envVarAction {
	return envVarAction{
		Remove: true,
		Var:    env,
	}
}

type envVarAction struct {
	Add    bool
	Remove bool
	Var    corev1.EnvVar
}

func (e Changes) Merge(container *corev1.Container) error {
	if container.Env == nil {
		container.Env = []corev1.EnvVar{}
	}

	if container.Env == nil {
		container.Env = []corev1.EnvVar{}
	}

	envVars := map[string]corev1.EnvVar{}

	for _, env := range container.Env {
		envVars[env.Name] = env
	}

	for _, envIn := range e {
		if envIn.Add {
			envVars[envIn.Var.Name] = envIn.Var
		}
		if envIn.Remove {
			delete(envVars, envIn.Var.Name)
		}
	}

	container.Env = []corev1.EnvVar{}
	for _, v := range envVars {
		container.Env = append(container.Env, v)
	}

	sort.Slice(container.Env, func(a, b int) bool {
		return container.Env[a].Name < container.Env[b].Name
	})

	return nil
}
