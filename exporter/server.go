//   Copyright 2019 Johannes Kohnen
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package exporter

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewDefaultServer(x *Exporter) *http.Server {
	// TODO this is not nicely done
	x.WrapPromHandler(promhttp.Handler())

	mux := http.NewServeMux()
	mux.HandleFunc(x.c.PromEndpoint, x.PromHandler)
	mux.HandleFunc(x.c.HealthEndpoint, x.healthHandler)
	mux.HandleFunc(x.c.LivenessEndpoint, x.livenessHandler)

	s := &http.Server{
		Addr:        x.c.Listen,
		ReadTimeout: 3e9,
		Handler:     mux,
	}
	s.SetKeepAlivesEnabled(false)
	return s
}
