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

package authchecker

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
)

type AuthChecker struct {
	Err error

	Client   dynamic.ResourceInterface
	FilePath string
	FileData []byte

	Logger    logr.Logger
	RetryTime time.Duration

	mutex       sync.Mutex
	tokenReview *unstructured.Unstructured
}

func (c *AuthChecker) init() error {
	var err error
	c.FileData, err = os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return err
	}

	c.tokenReview = &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"audiences": nil,
				"token":     string(c.FileData),
			},
		},
	}

	return nil
}

func (c *AuthChecker) Start(ctx context.Context) error {
	err := c.init()

	if err != nil {
		return err
	}

	ticker := time.NewTicker(c.RetryTime)

	go func() {
		defer ticker.Stop()

		for {
			time.Sleep(wait.Jitter(1*time.Second, 0.5))

			err := c.CheckToken(ctx)

			if err != nil {
				c.Logger.Error(err, "failed to check token err")
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	return nil
}

func (c *AuthChecker) CheckToken(ctx context.Context) (err error) {
	c.tokenReview.Object["metadata"] = map[string]interface{}{}

	c.tokenReview, err = c.Client.Create(ctx, c.tokenReview, v1.CreateOptions{})
	if err != nil {
		c.Logger.Error(err, "failed to create a token review")
		c.SetErr(err)
		return
	}

	isAuthed := false
	if status, ok := c.tokenReview.Object["status"]; ok {
		if authenticated, ok := status.(map[string]interface{})["authenticated"]; ok {
			isAuthed = authenticated.(bool)
		}
	}

	if !isAuthed {
		err = fmt.Errorf("token at file expired %s=%s", "file", c.FilePath)
		c.Logger.Error(err, "failed to get secret")
		c.SetErr(err)
		return
	}

	c.Clear()
	return
}

func (c *AuthChecker) SetErr(err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.Err = err
}

func (c *AuthChecker) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.Err = nil
}

func (c *AuthChecker) Check(_ *http.Request) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.Err != nil {
		return c.Err
	}

	return nil
}
