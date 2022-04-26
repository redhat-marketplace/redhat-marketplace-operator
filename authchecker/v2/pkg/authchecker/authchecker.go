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
	"net/http"
	"os"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AuthCheckChecker struct {
	Err error

	Client   client.Client
	FilePath string
	FileData []byte

	Logger    logr.Logger
	RetryTime time.Duration

	tokenReview *authv1.TokenReview

	mutex sync.Mutex
}

func (c *AuthCheckChecker) init() error {
	var err error
	c.FileData, err = os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")

	if err != nil {
		return err
	}

	c.tokenReview = &authv1.TokenReview{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: authv1.TokenReviewSpec{
			Audiences: nil,
			Token:     string(c.FileData),
		},
	}

	return nil
}

func (c *AuthCheckChecker) Start(ctx context.Context) error {
	err := c.init()

	if err != nil {
		return err
	}

	ticker := time.NewTicker(c.RetryTime)
	go func() {
		defer ticker.Stop()

		c.CheckToken(ctx)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	return nil
}

func (c *AuthCheckChecker) CheckToken(ctx context.Context) {
	// reset obj
	c.tokenReview.ObjectMeta = metav1.ObjectMeta{}

	err := c.Client.Create(ctx, c.tokenReview)
	if err != nil {
		c.Logger.Error(err, "failed to create a token review")
		c.SetErr(err)
		return
	}

	if !c.tokenReview.Status.Authenticated {
		err = errors.NewWithDetails("token at file expired", "file", c.FilePath)
		c.Logger.Error(err, "failed to get secret")
		c.SetErr(err)
		return
	}

	c.Clear()
	return
}

func (c *AuthCheckChecker) SetErr(err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.Err = err
}

func (c *AuthCheckChecker) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.Err = nil
}

func (c *AuthCheckChecker) Check(_ *http.Request) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.Err != nil {
		return c.Err
	}

	return nil
}
