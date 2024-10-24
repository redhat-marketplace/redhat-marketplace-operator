// Copyright 2024 IBM Corp.
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

package logger

import (
	"github.com/go-logr/logr"
	"github.com/hashicorp/go-retryablehttp"
	ctrl "sigs.k8s.io/controller-runtime"
)

type logLevel int

const (
	logLevelWarn  logLevel = iota
	logLevelInfo  logLevel = iota
	logLevelDebug logLevel = iota
)

type logger struct {
	logr.Logger
}

func (l logLevel) Level() int {
	return int(l)
}

func NewRetryableHTTPLogger() *logger {
	return &logger{
		Logger: ctrl.Log.WithName("retryablehttp"),
	}
}

var _ retryablehttp.LeveledLogger = (*logger)(nil)
var _ retryablehttp.Logger = (*logger)(nil)

func (l *logger) WithField(key string, value interface{}) *logger {
	return l.WithFields(key, value)
}

func (l *logger) WithError(err error) *logger {
	return l.WithFields("error", err)
}

func (l *logger) WithFields(keysAndValues ...interface{}) *logger {
	newLogger := *l
	newLogger.Logger = l.Logger.WithValues(keysAndValues...)
	return &newLogger
}

func (l *logger) Error(msg string, keysAndValues ...interface{}) {
	l.Logger.Error(nil, msg, keysAndValues...)
}

func (l *logger) Info(msg string, keysAndValues ...interface{}) {
	l.Logger.V(logLevelInfo.Level()).Info(msg, keysAndValues...)
}

func (l *logger) Debug(msg string, keysAndValues ...interface{}) {
	l.Logger.V(logLevelDebug.Level()).Info(msg, keysAndValues...)
}

func (l *logger) Warn(msg string, keysAndValues ...interface{}) {
	l.Logger.V(logLevelWarn.Level()).Info(msg, keysAndValues...)
}

func (l *logger) Printf(msg string, keysAndValues ...interface{}) {
	l.Logger.Info(msg, keysAndValues...)
}
