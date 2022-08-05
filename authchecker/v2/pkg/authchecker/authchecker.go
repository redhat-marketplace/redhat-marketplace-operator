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
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
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
	return err
}

func (c *AuthChecker) Start(ctx context.Context) error {
	err := c.init()

	if err != nil {
		return err
	}

	var timer *time.Timer

	// if we have an error, retry more often;
	// if no error, use a long retry time (default 5 mins)
	newTimer := func() *time.Timer {
		if c.HasErr() {
			return time.NewTimer(wait.Jitter(30*time.Second, 0.5))
		}

		return time.NewTimer(wait.Jitter(c.RetryTime, 0.25))
	}

	go func() {
		defer func() {
			if timer != nil {
				timer.Stop()
			}
		}()

		for {
			err := c.CheckToken(ctx)

			if err != nil {
				c.Logger.Error(err, "failed to check token err")
			}

			timer = newTimer()

			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				timer.Stop()
				timer = nil
				// jitter to make multiple checks happen randomly
			}
		}
	}()

	return nil
}

func (c *AuthChecker) CheckToken(ctx context.Context) (err error) {
	if c.tokenReview == nil {
		c.tokenReview = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{},
				"spec": map[string]interface{}{
					"audiences": nil,
					"token":     string(c.FileData),
				},
			},
		}
	} else {
		c.tokenReview.Object["metadata"] = map[string]interface{}{}
	}

	err = retry.OnError(retry.DefaultBackoff, func(error) bool {
		if errors.IsServerTimeout(err) {
			return true
		}
		if errors.IsTimeout(err) {
			return true
		}

		return false
	}, func() error {
		c.tokenReview, err = c.Client.Create(ctx, c.tokenReview, v1.CreateOptions{})
		return err
	})

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

func (c *AuthChecker) HasErr() bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.Err != nil
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
