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
	"github.com/prometheus/client_golang/prometheus/promhttp"
	flag "github.com/spf13/pflag"
)

var (
	startFile        = flag.String("file-start", "/var/run/fileage-exporter/start", "the start file")
	endFile          = flag.String("file-end", "/var/run/fileage-exporter/end", "the end-file")
	format           = flag.String("format", "2006-01-02 15:04:05.999999999-07:00", "the date parse format, defaults to what `date --rfc-3339=ns` puts out")
	hostPort         = flag.String("listen", ":9676", "host:port to listen at")
	promEndpoint     = flag.String("prom", "/metrics", "publish prometheus metrics on this URL endpoint")
	healthEndpoint   = flag.String("health", "/healthz", "publish health status on this URL endpoint")
	livenessEndpoint = flag.String("live", "/live", "publish liveness status on this URL endpoint")
	healthTimeout    = flag.Duration("health-timeout", 10*time.Minute, "when should the service considered unhealthy")
	welpenschutz     = flag.Duration("health-welpenschutz", 10*time.Minute, "how long initially the service is considered healthy.")
	livenessTimeout  = flag.Duration("liveness-timeout", 10*time.Minute, "when should the service considered un-live")
	loopDuration     = flag.Duration("loop", 2500*time.Millisecond, "how often to check files")
	namespace        = flag.String("namespace", "", "prometheus namespace")
	startup          = time.Now()

	mu              sync.RWMutex
	theStart        time.Time
	theEnd          time.Time
	theOld          time.Time
	endFileSeenOnce bool

	promUpdateCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: *namespace,
		Name:      "update_count_total",
		Help:      "Counter of update runs.",
	})
	promUpdateAge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: *namespace,
		Name:      "last_update_age_seconds",
		Help:      "Time since last time an update finished.",
	})
	promUpdateDuration = prometheus.NewSummary(prometheus.SummaryOpts{
		Namespace: *namespace,
		Name:      "update_duration_seconds",
		Help:      "Duration of update runs in seconds.",
	})
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options...]\n\n  TODO\n  Options:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	prometheus.MustRegister(promUpdateCount)
	prometheus.MustRegister(promUpdateAge)
	prometheus.MustRegister(promUpdateDuration)
	http.Handle(*promEndpoint, promhttp.Handler())
	http.HandleFunc(*healthEndpoint, healthHandler)
	http.HandleFunc(*livenessEndpoint, livenessHandler)
}

func main() {
	loopMeasure()
	err := http.ListenAndServe(*hostPort, nil)
	log.Fatal(err)
}

func loopMeasure() {
	go func() {
		for {
			start, end := measure(*startFile), measure(*endFile)
			update(start, end)

			time.Sleep(*loopDuration)
		}
	}()
}

func update(start, end time.Time) {
	mu.Lock()
	defer mu.Unlock()

	theStart = start
	theEnd = end

	promUpdateAge.Set(time.Since(end).Seconds())

	if end.IsZero() || theOld == end {
		return
	}
	theOld = end
	endFileSeenOnce = true

	promUpdateCount.Inc()

	if !start.IsZero() && !end.IsZero() {
		dur := end.Sub(start)
		if dur < 0 {
			panic(fmt.Errorf("Duration negative, start %s, end %s, dur %s", start, end, dur))
		}
		promUpdateDuration.Observe(dur.Seconds())
	}
}

// in case of error returns zero time.Time
func measure(filename string) time.Time {
	if filename == "" {
		return time.Time{}
	}

	if *format != "" {
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			return time.Time{}
		}
		t, err := time.Parse(*format, strings.TrimSpace(string(data)))
		if err == nil {
			return t
		}
		log.Printf("Fall back to mtime for file: %s", filename)
	}

	stat, err := os.Stat(filename)
	if err != nil {
		return time.Time{}
	}
	return stat.ModTime()
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeStatusReponse(w, *healthTimeout, *welpenschutz)
}

func livenessHandler(w http.ResponseWriter, r *http.Request) {
	writeStatusReponse(w, *livenessTimeout, 0)
}

func writeStatusReponse(w http.ResponseWriter, timeout, welpenschutz time.Duration) {
	mu.RLock()
	myEnd := theEnd
	mu.RUnlock()

	age := time.Since(myEnd)
	good := age < timeout

	maturiy := myEnd.Sub(startup)
	if welpenschutz > 0 && maturiy < welpenschutz {
		good = true
	}
	if good {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	fmt.Fprintf(w, "last_update: %s\r\n", myEnd.UTC().Format(time.RFC3339Nano))
	fmt.Fprintf(w, "# time %s means never.\r\n", time.Time{})
}
