package prometheus_fileage_exporter

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewDefaultServer(x *Exporter) *http.Server {
	//TODO this is not nicely done
	x.WrapPromHandler(promhttp.Handler())

	mux := http.NewServeMux()
	mux.HandleFunc(x.c.PromEndpoint, x.PromHandler)
	mux.HandleFunc(x.c.HealthEndpoint, x.healthHandler)

	s := &http.Server{
		Addr:        x.c.Listen,
		ReadTimeout: 3e9,
		Handler:     mux,
	}
	s.SetKeepAlivesEnabled(false)
	return s
}
