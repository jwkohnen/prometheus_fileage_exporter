//   Copyright 2016 Wolfgang Johannes Kohnen
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
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	flag "github.com/spf13/pflag"
)

var (
	startFile        = flag.String("file-start", "/var/run/fileage-exporter/start", "the start file")
	endFile          = flag.String("file-end", "/var/run/fileage-exporter/end", "the end-file")
	format           = flag.String("format", "2006-01-02 15:04:05.999999999-07:00", "the date parse format, defaults to what `date --rfc-3339=ns` puts out")
	hostPort         = flag.String("listen", "localhost:9676", "host:port to listen at")
	promEndpoint     = flag.String("prom", "/metrics", "publish prometheus metrics on this URL endpoint")
	healthEndpoint   = flag.String("health", "/healthz", "publish health status on this URL endpoint")
	livenessEndpoint = flag.String("live", "/live", "publish liveness status on this URL endpoint")
	healthTimeout    = flag.Duration("health-timeout", 10*time.Minute, "when should the service considered unhealthy")
	initDuration     = flag.Duration("init-duration", 15*time.Minute, "when should the service considered unhealthy initially")
	loopDuration     = flag.Duration("loop", 2500*time.Millisecond, "how often to check files")
	namespace        = flag.String("namespace", "", "prometheus namespace")
	hostname         = "unknown"

	mu              sync.RWMutex
	start           time.Time
	end             time.Time
	old             time.Time
	endFileSeenOnce bool

	promUpdateCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: *namespace,
		Name:      "update_count_total",
		Help:      "Counter of update runs.",
	}, []string{"hostname"})
	promLastUpdate = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: *namespace,
		Name:      "last_update_time",
		Help:      "Timestamp of last time an update finished",
	})
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "lasdaklsdj")
		flag.PrintDefaults()
	}
	flag.Parse()
	h, err := os.Hostname()
	if err == nil {
		hostname = h
	}
	prometheus.MustRegister(promUpdateCount)
	prometheus.MustRegister(promLastUpdate)
	http.Handle(*promEndpoint, prometheus.Handler())
	http.HandleFunc(*healthEndpoint, healthHandler)
	http.HandleFunc(*livenessEndpoint, livenessHandler)
}

func main() {
	fmt.Println("starting main")
	loopMeasure()
	log.Fatal(http.ListenAndServe(*hostPort, nil))
}

func loopMeasure() {
	fmt.Println("starting")
	go func() {
		for {
			mu.Lock()
			start, end = measure(*startFile), measure(*endFile)
			update(start, end)
			mu.Unlock()

			time.Sleep(*loopDuration)
		}
	}()
}

func update(start, end time.Time) {
	if end.IsZero() || old == end {
		return
	}
	endFileSeenOnce = true
	promUpdateCount.WithLabelValues(hostname).Inc()
	promLastUpdate.Set(float64(end.Unix()))

	old = end
}

// in case of error returns zero time.Time
func measure(file string) time.Time {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return time.Time{}
	}
	t, err := time.Parse(*format, strings.TrimSpace(string(data)))
	if err == nil {
		return t
	}

	log.Printf("Fall back to mtime for file: %s", file)
	stat, err := os.Stat(file)
	if err != nil {
		return time.Time{}
	}
	return stat.ModTime()
}

func healthHandler(w http.ResponseWriter, r *http.Request)   {}
func livenessHandler(w http.ResponseWriter, r *http.Request) {}
