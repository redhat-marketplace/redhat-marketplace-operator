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

package logger

import (
	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	InfoLevel  = 1
	DebugLevel = 5
	TraceLevel = 10
)

type Logger struct {
	logr.Logger
}

func NewLogger(name string) *Logger {
	return &Logger{
		Logger: logf.Log.WithName(name),
	}
}

func (l *Logger) NewRequestLogger(request reconcile.Request) *Logger {
	return &Logger{
		Logger: l.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name),
	}
}

func (l *Logger) Info(msg string, keysAndValues ...interface{}) {
	l.V(InfoLevel).Info(msg, append(keysAndValues, "level", "info")...)
}

func (l *Logger) Debug(msg string, keysAndValues ...interface{}) {
	l.V(DebugLevel).Info(msg, append(keysAndValues, "level", "debug")...)
}

func (l *Logger) Trace(msg string, keysAndValues ...interface{}) {
	l.V(TraceLevel).Info(msg, append(keysAndValues, "level", "trace")...)
}

func SetLoggerToDevelopmentZap() {
	logf.SetLogger(zap.New(zap.UseDevMode(true)))
}
