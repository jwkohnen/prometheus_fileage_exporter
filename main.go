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
	"flag"
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
)

var (
	startFile        = flag.String("file-start", "", "the start file")
	endFile          = flag.String("file-end", "", "the end-file")
	hostPort         = flag.String("listen", ":9676", "host:port to listen at")
	promEndpoint     = flag.String("prom", "/metrics", "publish prometheus metrics on this URL endpoint")
	healthEndpoint   = flag.String("health", "/healthz", "publish health status on this URL endpoint")
	livenessEndpoint = flag.String("liveness", "/liveness", "publish liveness status on this URL endpoint")
	healthTimeout    = flag.Duration("health-timeout", 10*time.Minute, "when should the service be considered unhealthy")
	livenessTimeout  = flag.Duration("liveness-timeout", 10*time.Minute, "when should the service be considered un-live")
	welpenschutz     = flag.Duration("health-welpenschutz", 10*time.Minute, "how long initially the service is considered healthy.")
	directoryTimeout = flag.Duration("directory-timeout", 10*time.Minute, "how long to wait for missing directories")
	namespace        = flag.String("namespace", "", "prometheus namespace")

	promHandler = promhttp.Handler()
	startup     = time.Now()

	theMu     sync.RWMutex
	theStart  time.Time
	theEnd    time.Time
	theOldEnd time.Time

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
	promUpdateRunning = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: *namespace,
		Name:      "update_running",
		Help:      "If the monitored process seems to run: 0 no; 1 yes.",
	})
	promUpdateDuration = prometheus.NewSummary(prometheus.SummaryOpts{
		Namespace: *namespace,
		Name:      "update_duration_seconds",
		Help:      "Duration of update runs in seconds.",
	})
	onceRegisterUpdateRunning  sync.Once
	onceRegisterUpdateDuration sync.Once
	onceRegisterUpdateAge      sync.Once
)

func init() {
	flag.Parse()
	if flag.NArg() != 0 {
		log.Fatalf("Superfluous arguments: %v", flag.Args())
	}

	prometheus.MustRegister(promUpdateCount)

	http.HandleFunc(*promEndpoint, promHandlerWrapper)
	http.HandleFunc(*healthEndpoint, healthHandler)
	http.HandleFunc(*livenessEndpoint, livenessHandler)

	var err error
	if *startFile != "" {
		*startFile, err = filepath.Abs(*startFile)
		if err != nil {
			log.Fatal(err)
		}
	}
	if *endFile == "" {
		log.Fatalln("--end-file must be set!")
	}
	*endFile, err = filepath.Abs(*endFile)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	startWatcher, endWatcher := createWatcher(*startFile), createWatcher(*endFile)
	watch(startWatcher, endWatcher)
	log.Fatal(http.ListenAndServe(*hostPort, nil))
}

func createWatcher(filename string) *fsnotify.Watcher {
	if filename == "" {
		// return a watcher that will block forever
		return &fsnotify.Watcher{}
	}

	deadline := startup.Add(*directoryTimeout)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Error creating fs notifier: %v", err)
	}
	d := filepath.Dir(filename)
	for {
		addErr := w.Add(d)
		if addErr == nil {
			break
		}
		if addErr != nil {
			if time.Now().After(deadline) {
				log.Fatalf("Giving up adding directory \"%s\": %v", addErr)
			}
			log.Printf("Retrying to add directory \"%s\" after error: %v", d, addErr)
		}
	}
	return w
}

func watch(startWatcher, endWatcher *fsnotify.Watcher) {
	go func() {
		bs := filepath.Base(*startFile)
		be := filepath.Base(*endFile)

		update()
		for {
			select {
			case e := <-startWatcher.Events:
				if filepath.Base(e.Name) == bs {
					update()
				}
			case e := <-endWatcher.Events:
				if filepath.Base(e.Name) == be {
					update()
				}
			case err := <-startWatcher.Errors:
				log.Printf("Error waiting for fs event on start file: %v", err)
			case err := <-endWatcher.Errors:
				log.Printf("Error waiting for fs event on end file: %v", err)
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

	theMu.Lock()
	defer theMu.Unlock()

	theStart, theEnd = start, end

	if !start.IsZero() {
		onceRegisterUpdateRunning.Do(func() { prometheus.MustRegister(promUpdateRunning) })
		if end.IsZero() || start.After(end) {
			log.Println("An update run started.")
			promUpdateRunning.Set(1)
		} else {
			promUpdateRunning.Set(0)
		}
	}

	if !end.IsZero() && end != theOldEnd {
		theOldEnd = end
		if start.After(end) || startup.After(end) {
			return
		}
		log.Println("An update run ended.")
		promUpdateCount.Inc()
		if !start.IsZero() {
			onceRegisterUpdateDuration.Do(func() { prometheus.MustRegister(promUpdateDuration) })
			promUpdateDuration.Observe(end.Sub(start).Seconds())
		}
	}
}

// promHandlerWrapper updates update_age just before handling scrape
func promHandlerWrapper(w http.ResponseWriter, r *http.Request) {
	theMu.RLock()
	myEnd := theEnd
	theMu.RUnlock()

	if !myEnd.IsZero() {
		onceRegisterUpdateAge.Do(func() { prometheus.MustRegister(promUpdateAge) })
		age := time.Since(myEnd)
		promUpdateAge.Set(age.Seconds())
	}
	promHandler.ServeHTTP(w, r)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeStatusReponse(w, *healthTimeout, *welpenschutz)
}

func livenessHandler(w http.ResponseWriter, r *http.Request) {
	writeStatusReponse(w, *livenessTimeout, 0)
}

func writeStatusReponse(w http.ResponseWriter, timeout, welpenschutz time.Duration) {
	theMu.RLock()
	myEnd := theEnd
	theMu.RUnlock()

	updateAge := time.Since(myEnd)
	good := updateAge < timeout
	if welpenschutz > 0 && time.Since(startup) < welpenschutz {
		good = true
	}

	const body = "last_update: %s\r\n" +
		"# time %s means never.\r\n" +
		"# alive/healhty: %t\r\n"
	endF := myEnd.Format(time.RFC3339Nano)
	if good {
		fmt.Fprintf(w, fmt.Sprintf(body, endF, time.Time{}, good))
	} else {
		http.Error(w, fmt.Sprintf(body, endF, time.Time{}, good), http.StatusServiceUnavailable)
	}
}
