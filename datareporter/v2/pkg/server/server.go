package server

import (
	"encoding/json"
	"io"
	"net/http"

	emperror "emperror.dev/errors"
	datareporterv1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/api/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/events"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/version"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("events_api_handler")

func NewDataReporterHandler(eventEngine *events.EventEngine, eventConfig *events.Config, handlerConfig datareporterv1alpha1.ApiHandlerConfig) http.Handler {
	router := http.NewServeMux()

	router.HandleFunc("/v1/event", func(w http.ResponseWriter, r *http.Request) {
		EventHandler(eventEngine, eventConfig, w, r)
	})

	router.HandleFunc("/v1/status", func(w http.ResponseWriter, r *http.Request) {
		StatusHandler(w, r)
	})

	muxWithMiddleware := http.TimeoutHandler(router, handlerConfig.HandlerTimeout.Duration, "timeout exceeded")

	return muxWithMiddleware
}

func EventHandler(eventEngine *events.EventEngine, eventConfig *events.Config, w http.ResponseWriter, r *http.Request) {
	log.Info("v1/event")
	headerAPIKey := r.Header.Get("apiKey")

	if !eventConfig.ApiKeys.HasKey(events.Key(headerAPIKey)) {
		w.WriteHeader(http.StatusBadRequest)
		err := emperror.New("api key not found on DataReporterConfig cr")
		log.Error(err, "error validating api key")
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
	event := events.Event{Key: events.Key(headerAPIKey), RawMessage: rawMessage}

	eventEngine.EventChan <- event

	log.Info("event sent to event engine", "event", event)

	w.WriteHeader(http.StatusOK)
	out, _ := json.Marshal(event)
	w.Write(out)
}

type StatusResponse struct {
	Name    string
	Version string
}

func StatusHandler(w http.ResponseWriter, r *http.Request) {
	log.Info("v1/status")

	w.WriteHeader(http.StatusOK)

	statusRes := StatusResponse{
		Name:    "Data Reporter Operator",
		Version: version.Version,
	}

	statusString, _ := json.Marshal(statusRes)

	w.Write([]byte(statusString))
}
