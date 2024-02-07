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
	"encoding/json"
	"io"
	"net/http"
	"net/http/pprof"
	"os"

	emperror "emperror.dev/errors"
	datareporterv1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/api/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/datafilter"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/events"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/version"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("events_api_handler")

func NewDataReporterHandler(eventEngine *events.EventEngine, eventConfig *events.Config, dataFilters *datafilter.DataFilters, handlerConfig datareporterv1alpha1.ApiHandlerConfig) http.Handler {
	router := http.NewServeMux()

	router.HandleFunc("/v1/event", func(w http.ResponseWriter, r *http.Request) {
		EventHandler(eventEngine, eventConfig, dataFilters, w, r)
	})

	router.HandleFunc("/v1/status", func(w http.ResponseWriter, r *http.Request) {
		StatusHandler(w, r)
	})

	// if debug enabled
	if debug := os.Getenv("PPROF_DEBUG"); debug == "true" {
		router.HandleFunc("/debug/pprof/", pprof.Index)
		router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		router.HandleFunc("/debug/pprof/profile", pprof.Profile)
		router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		router.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	muxWithMiddleware := http.TimeoutHandler(router, handlerConfig.HandlerTimeout.Duration, "timeout exceeded")

	return muxWithMiddleware
}

func EventHandler(eventEngine *events.EventEngine, eventConfig *events.Config, dataFilters *datafilter.DataFilters, w http.ResponseWriter, r *http.Request) {
	log.WithName("events_api_handler v1/event")

	if !eventEngine.IsReady() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	if eventConfig.LicenseAccept != true {
		w.WriteHeader(http.StatusInternalServerError)
		err := emperror.New("license has not been accepted in marketplaceconfig. event handler will not accept events.")
		log.Error(err, "error with configuration")
		return
	}

	headerUser := r.Header.Get("x-remote-user")
	if headerUser == "" {
		w.WriteHeader(http.StatusBadRequest)
		err := emperror.New("request received without x-remote-user header")
		log.Error(err, "error with request header")
		return
	}

	eventKeyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Error(err, "error reading request body")
		return
	}

	if !json.Valid(eventKeyBytes) {
		w.WriteHeader(http.StatusBadRequest)
		err = emperror.New("event is not valid json")
		log.Error(err, "error validating event json")
		return
	}

	rawMessage := json.RawMessage(eventKeyBytes)
	event := events.Event{User: headerUser, RawMessage: rawMessage}

	// Event is sent to EventProcessor accumulator
	// Formatted as Report
	// Sent to DataService
	// There is no guaranttee of receipt
	eventEngine.EventChan <- event

	log.V(4).Info("event sent to event engine", "event", event)

	// Send to DataFilters
	// Events are sent to to DataFilter Destinations with confirmation before sending StatusOK
	statusCodes := dataFilters.FilterAndUpload(event)

	for _, statusCode := range statusCodes {
		if statusCode != http.StatusOK {
			w.WriteHeader(statusCode)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}
}

type StatusResponse struct {
	Name    string
	Version string
}

func StatusHandler(w http.ResponseWriter, r *http.Request) {
	log.WithName("events_api_handler v1/status")

	w.WriteHeader(http.StatusOK)

	statusRes := StatusResponse{
		Name:    "Data Reporter Operator",
		Version: version.Version,
	}

	statusString, _ := json.Marshal(statusRes)

	w.Write([]byte(statusString))
}
