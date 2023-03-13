// Copyright 2023 IBM Corp.
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
	"net"
	"net/http"
	"strconv"
	"time"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("meteric_generator")

type DataReporterHandler struct {
	WriteTimeout time.Duration
	ReadTimeout  time.Duration
}

func NewDataReporterHandler() *DataReporterHandler {
	return &DataReporterHandler{
		WriteTimeout: time.Second * 30,
		ReadTimeout:  time.Second * 30,
	}
}

func (s *DataReporterHandler) Serve(host string, port int) {
	listenAddress := net.JoinHostPort(host, strconv.Itoa(port))

	log.Info("Starting data reporter api handler", "listenAddress", listenAddress)

	mux := http.NewServeMux()

	mux.HandleFunc("/v1/event", eventHandler)
	mux.HandleFunc("/v1/status", statusHandler)

	err := http.ListenAndServe(listenAddress, mux)
	if err != nil {
		log.Error(err, "failing to start data reporter api handler")
		panic(err)
	}
}

func eventHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("event handler"))
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(http.StatusText(http.StatusOK)))
}
