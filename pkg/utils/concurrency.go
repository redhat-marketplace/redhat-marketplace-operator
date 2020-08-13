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

package utils

import (
	"time"

	"github.com/jpillora/backoff"
)

func Retry(f func() error, retry int) error {
	var err error

	jitter := &backoff.Backoff{
		Jitter: true,
	}

	err = f()

	if err == nil {
		return nil
	}

	for i := 0; i < retry; i++ {
		err = f()

		if err == nil {
			return nil
		}

		b := jitter.Duration()
		time.Sleep(b)
	}

	if err != nil {
		return err
	}

	return nil
}
