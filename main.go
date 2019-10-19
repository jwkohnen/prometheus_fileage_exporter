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

package main

import (
	"flag"
	"os"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/jwkohnen/prometheus_fileage_exporter/exporter"
)

func main() {
	// Prepare logging
	log := logrus.New()
	log.Out = os.Stderr

	s := exporter.NewDefaultServer(exporter.NewExporter(configure(log), log))

	log.Fatal(s.ListenAndServe())
}

func configure(log *logrus.Logger) *exporter.Config {
	config := &exporter.Config{}
	flag.StringVar(&config.StartFile, "file-start", "",
		"the start file",
	)
	flag.StringVar(&config.EndFile, "file-end", "",
		"the end-file",
	)
	flag.StringVar(&config.Listen, "listen", ":9104",
		"host:port to listen at",
	)
	flag.StringVar(&config.PromEndpoint, "prom", "/metrics",
		"publish prometheus metrics on this URL endpoint",
	)
	flag.StringVar(&config.HealthEndpoint, "health", "/healthz",
		"publish health status on this URL endpoint",
	)
	flag.StringVar(&config.LivenessEndpoint, "liveness", "/liveness",
		"publish liveness status on this URL endpoint",
	)
	flag.StringVar(&config.Namespace, "namespace", "",
		"prometheus namespace",
	)
	flag.StringVar(&config.Subsystem, "subsystem", "",
		"prometheus subsystem",
	)
	flag.DurationVar(&config.HealthTimeout, "health-timeout", 10*time.Minute,
		"when should the service be considered unhealthy",
	)
	flag.DurationVar(&config.LivenessTimeout, "liveness-timeout", 10*time.Minute,
		"when should the service be considered un-live",
	)
	flag.DurationVar(&config.Welpenschutz, "health-welpenschutz", 10*time.Minute,
		"how long initially the service is considered healthy.",
	)
	flag.DurationVar(&config.DirectoryTimeout, "directory-timeout", 10*time.Minute,
		"how long to wait for missing directories",
	)
	flag.BoolVar(&config.Debug, "debug", true,
		"enable debug logging (enabled by default)",
	)
	flag.BoolVar(&config.LogJSON, "log-json", false,
		"enable JSON-formatted logging",
	)
	flag.Parse()

	if config.LogJSON {
		log.Formatter = new(logrus.JSONFormatter)
	}

	if flag.NArg() != 0 {
		log.Fatalf("Superfluous arguments: %v", flag.Args())
	}

	return config
}
