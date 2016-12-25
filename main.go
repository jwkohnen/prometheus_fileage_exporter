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
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	flag "github.com/spf13/pflag"
)

var (
	startFile        = flag.String("file-start", "/var/run/fileage-exporter/start", "the start file")
	endFile          = flag.String("file-end", "/var/run/fileage-exporter/end", "the end-file")
	hostPort         = flag.String("listen", ":9676", "host:port to listen at")
	promEndpoint     = flag.String("prom", "/metrics", "publish prometheus metrics on this URL endpoint")
	healthEndpoint   = flag.String("health", "/healthz", "publish health status on this URL endpoint")
	livenessEndpoint = flag.String("liveness", "/liveness", "publish liveness status on this URL endpoint")
	healthTimeout    = flag.Duration("health-timeout", 10*time.Minute, "when should the service considered unhealthy")
	welpenschutz     = flag.Duration("health-welpenschutz", 10*time.Minute, "how long initially the service is considered healthy.")
	livenessTimeout  = flag.Duration("liveness-timeout", 10*time.Minute, "when should the service considered un-live")
	namespace        = flag.String("namespace", "", "prometheus namespace")
	startup          = time.Now()

	mu          sync.RWMutex
	theStart    time.Time
	theEnd      time.Time
	theOld      time.Time
	promHandler http.Handler

	promUpdateCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: *namespace,
		Name:      "update_count_total",
		Help:      "Counter of update runs.",
	})
	promUpdateAge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: *namespace,
		Name:      "update_age_seconds",
		Help:      "Time since last time an update finished.",
	})
	promUpdateDuration = prometheus.NewSummary(prometheus.SummaryOpts{
		Namespace: *namespace,
		Name:      "update_duration_seconds",
		Help:      "Duration of update runs in seconds.",
	})
)

func init() {
	flag.Parse()
	prometheus.MustRegister(promUpdateCount)
	prometheus.MustRegister(promUpdateAge)
	prometheus.MustRegister(promUpdateDuration)
	promHandler = promhttp.Handler()
	http.HandleFunc(*promEndpoint, promHandlerWrapper)
	http.HandleFunc(*healthEndpoint, healthHandler)
	http.HandleFunc(*livenessEndpoint, livenessHandler)
}

func main() {
	watch()
	log.Fatal(http.ListenAndServe(*hostPort, nil))
}

func watch() {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("Error creating fs notifier:", err)
	}
	err = w.Add(filepath.Dir(*endFile))
	if err != nil {
		log.Fatal("Error adding dir to watcher:", err)
	}

	go func() {
		update()
		for {
			select {
			case e := <-w.Events:
				if filepath.Base(e.Name) != filepath.Base(*endFile) {
					continue
				}
				update()
			case err := <-w.Errors:
				log.Println("Error waiting for fs event:", err)
			}
		}
	}()
}

// in case of error returns zero time.Time
func measure(filename string) (mtime time.Time) {
	if filename == "" {
		return
	}
	stat, err := os.Stat(filename)
	if err != nil {
		return
	}
	return stat.ModTime()
}

func update() {
	start, end := measure(*startFile), measure(*endFile)

	mu.Lock()
	defer mu.Unlock()

	theStart = start
	theEnd = end

	if end.IsZero() || theOld == end {
		return
	}

	theOld = end
	promUpdateCount.Inc()

	if start.IsZero() || end.IsZero() {
		return
	}

	dur := end.Sub(start)
	if dur < 0 {
		panic(fmt.Errorf("Duration negative, start %s, end %s, dur %s", start, end, dur))
	}
	promUpdateDuration.Observe(dur.Seconds())
}

// promHandlerWrapper updates update_age just before handling scrape
func promHandlerWrapper(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	age := time.Since(theEnd)
	mu.RUnlock()

	promUpdateAge.Set(age.Seconds())
	promHandler.ServeHTTP(w, r)
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

	updateAge := time.Since(myEnd)
	good := updateAge < timeout
	if welpenschutz > 0 && time.Since(startup) < welpenschutz {
		good = true
	}

	body := fmt.Sprintf("last_update: %s\r\n"+
		"# time %s means never.\r\n",
		myEnd.Format(time.RFC3339Nano),
		time.Time{})
	if good {
		fmt.Fprint(w, body)
	} else {
		http.Error(w, body, http.StatusServiceUnavailable)
	}
}
