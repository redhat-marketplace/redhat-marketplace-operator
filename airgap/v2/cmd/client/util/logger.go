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

package util

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

//InitLog initializes logger and returns instance of logger and error if any
func InitLog() (logr.Logger, error) {
	var log logr.Logger
	zapLog, err := zap.NewDevelopment()
	if err != nil {
		return log, fmt.Errorf("failed to initialize zapr, due to error: %v", err)
	}
	log = zapr.NewLogger(zapLog)
	// enable/disable logging
	if !viper.GetBool("verbose") {
		log = logr.Discard()
	}
	return log, nil
}
