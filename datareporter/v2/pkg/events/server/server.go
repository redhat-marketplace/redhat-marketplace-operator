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

	log.Info("Starting metrics server", "listenAddress", listenAddress)

	mux := http.NewServeMux()

	mux.HandleFunc("/v1/event", eventHandler)
	mux.HandleFunc("/v1/status", statusHandler)

	err := http.ListenAndServe(listenAddress, mux)
	if err != nil {
		log.Error(err, "failing to listen and serve")
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
