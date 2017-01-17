//   Copyright 2016 Johannes Kohnen
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
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/prometheus/client_golang/prometheus"
)

type Exporter struct {
	c                          *Config
	promUpdateCount            prometheus.Counter
	promUpdateAge              prometheus.Gauge
	promUpdateRunning          prometheus.Gauge
	promUpdateDuration         prometheus.Summary
	onceRegisterUpdateRunning  sync.Once
	onceRegisterUpdateDuration sync.Once
	onceRegisterUpdateAge      sync.Once
	startup                    time.Time
	promHandler                http.Handler

	mu     sync.RWMutex
	start  time.Time
	end    time.Time
	oldEnd time.Time
}

func NewExporter(c *Config) *Exporter {
	x := &Exporter{
		c:       c,
		startup: time.Now(),
		promUpdateCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: c.Namespace,
			Subsystem: c.Subsystem,
			Name:      "update_count_total",
			Help:      "Counter of update runs.",
		}),
		promUpdateAge: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: c.Namespace,
			Subsystem: c.Subsystem,
			Name:      "update_age_seconds",
			Help:      "Time since last time an update finished.",
		}),
		promUpdateRunning: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: c.Namespace,
			Subsystem: c.Subsystem,
			Name:      "update_running",
			Help:      "If the monitored process seems to run: 0 no; 1 yes.",
		}),
		promUpdateDuration: prometheus.NewSummary(prometheus.SummaryOpts{
			Namespace: c.Namespace,
			Subsystem: c.Subsystem,
			Name:      "update_duration_seconds",
			Help:      "Duration of update runs in seconds.",
		}),
	}
	prometheus.MustRegister(x.promUpdateCount)

	// FIXME: you shall not surprisingly modify a config object
	var err error
	if x.c.StartFile != "" {
		x.c.StartFile, err = filepath.Abs(x.c.StartFile)
		if err != nil {
			log.Fatal(err)
		}
	}
	if x.c.EndFile == "" {
		log.Fatalln("--end-file must be set!")
	}
	x.c.EndFile, err = filepath.Abs(x.c.EndFile)
	if err != nil {
		log.Fatal(err)
	}

	startWatcher, endWatcher := x.createWatcher(x.c.StartFile), x.createWatcher(x.c.EndFile)
	x.watch(startWatcher, endWatcher)

	return x
}

func (x *Exporter) WrapPromHandler(handler http.Handler) {
	x.promHandler = handler
}

func (x *Exporter) createWatcher(filename string) *fsnotify.Watcher {
	if filename == "" {
		// return a watcher that will block forever
		return &fsnotify.Watcher{}
	}

	deadline := x.startup.Add(x.c.DirectoryTimeout)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Error creating fs notifier: %v", err)
	}
	d := filepath.Dir(filename)
	deadline := time.NewTimer(x.startup.Add(x.c.DirectoryTimeout).Sub(time.Now()))
	for backoff := time.Second; ; backoff *= 2 {
		addErr := w.Add(d)
		if addErr == nil {
			break
		} else {
			select {
			case <-time.After(backoff):
				log.Printf("Retrying to add directory \"%s\" in %s after error: %v", d, backoff, addErr)
				continue
			case <-deadline.C:
				log.Fatalf("Giving up adding directory \"%s\": %v", d, addErr)
			}
		}
	}
	return w
}

func (x *Exporter) watch(startWatcher, endWatcher *fsnotify.Watcher) {
	go func() {
		bs := filepath.Base(x.c.StartFile)
		be := filepath.Base(x.c.EndFile)

		x.update()
		for {
			select {
			case e := <-startWatcher.Events:
				if filepath.Base(e.Name) == bs {
					x.update()
				}
			case e := <-endWatcher.Events:
				if filepath.Base(e.Name) == be {
					x.update()
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

func (x *Exporter) update() {
	start, end := measure(x.c.StartFile), measure(x.c.EndFile)

	x.mu.Lock()
	defer x.mu.Unlock()

	x.start, x.end = start, end

	if !start.IsZero() {
		x.onceRegisterUpdateRunning.Do(func() { prometheus.MustRegister(x.promUpdateRunning) })
		if end.IsZero() || start.After(end) {
			log.Println("An update run started.")
			x.promUpdateRunning.Set(1)
		} else {
			x.promUpdateRunning.Set(0)
		}
	}

	if !end.IsZero() && end != x.oldEnd {
		x.oldEnd = end
		if start.After(end) || x.startup.After(end) {
			return
		}
		log.Println("An update run ended.")
		x.promUpdateCount.Inc()
		if !start.IsZero() {
			x.onceRegisterUpdateDuration.Do(func() { prometheus.MustRegister(x.promUpdateDuration) })
			x.promUpdateDuration.Observe(end.Sub(start).Seconds())
		}
	}
}

// PromHandler updates update_age just before handling scrape
func (x *Exporter) PromHandler(w http.ResponseWriter, r *http.Request) {
	x.mu.RLock()
	myEnd := x.end
	x.mu.RUnlock()

	if !myEnd.IsZero() {
		x.onceRegisterUpdateAge.Do(func() { prometheus.MustRegister(x.promUpdateAge) })
		x.promUpdateAge.Set(time.Since(myEnd).Seconds())
	}
	x.promHandler.ServeHTTP(w, r)
}

func (x *Exporter) healthHandler(w http.ResponseWriter, r *http.Request) {
	x.writeStatusReponse(w, x.c.HealthTimeout, x.c.Welpenschutz)
}

func (x *Exporter) livenessHandler(w http.ResponseWriter, r *http.Request) {
	x.writeStatusReponse(w, x.c.LivenessTimeout, 0)
}

func (x *Exporter) writeStatusReponse(w http.ResponseWriter, timeout, welpenschutz time.Duration) {
	x.mu.RLock()
	myEnd := x.end
	x.mu.RUnlock()

	updateAge := time.Since(myEnd)
	good := updateAge < timeout
	if welpenschutz > 0 && time.Since(x.startup) < welpenschutz {
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
