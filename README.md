# Prometheus Fileage Exporter 
This package monitors a process of some sort--usually an update or batch process--and
exports metrics via a prometheus scrape endpoint.

This package has been designed with kubernetes pods as runtime environment in mind, but
may prove useful in other scenarios.

The monitoring is simply facilitated by watching timestamp files, a timestamp file that
signals that the process has finished and optionally a timestamp file that signals the
start of the process.

Only the `mtime` of the files is used, so a simple `touch` off a shell script will suffice
and is recommended.

There are two metrics if only an end file is provided:

 *  `update_count_total`: Counter of update runs.
 *  `update_age_seconds`: Gauge with time since last time an update finished.

If a start file is provided two additional metrics are provided:

 *  `update_running`: Gauge with a flag if an update is currently running (1) or not (0).
 *  `update_duration_seconds`: Summary of durations of update runs in seconds.

Additionally two HTTP endpoints report healthiness and liveness depending
on the age of the end file.

# Bugs and Limitations

The metrics will be skewed if the process touches a start file, then dies and picks up
running again with stale files lingering around. This shortcoming is due to the
kubernetes pod design, as a failing container (which does the actual work) will also tear
down the prometheus fileage exporter container as well as the timestamp files with it.

# Usage

```
Usage of ./prometheus-fileage-exporter:
  -directory-timeout duration
    	how long to wait for missing directories (default 10m0s)
  -file-end string
    	the end-file
  -file-start string
    	the start file
  -health string
    	publish health status on this URL endpoint (default "/healthz")
  -health-timeout duration
    	when should the service be considered unhealthy (default 10m0s)
  -health-welpenschutz duration
    	how long initially the service is considered healthy. (default 10m0s)
  -listen string
    	host:port to listen at (default ":9676")
  -liveness string
    	publish liveness status on this URL endpoint (default "/liveness")
  -liveness-timeout duration
    	when should the service be considered un-live (default 10m0s)
  -namespace string
    	prometheus namespace
  -prom string
    	publish prometheus metrics on this URL endpoint (default "/metrics")
```

# License
Copyright 2016 Wolfgang Johannes Kohnen

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

