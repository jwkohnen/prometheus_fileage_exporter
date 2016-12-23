# Prometheus Fileage Exporter 
This package exports simple stats via Prometheus when some process finished
up its job and the duration.

The process to be monitored needs to update some file with a timestamp like
the output of `date`. Alternatively the mtime of this file is used. If a
"start" file is provided, the duration time is measured as well. 

There are three metrics:

 *  `update_count_total` Counter of update runs.
 *  `last_update_age_seconds`  Time since last time an update finished.
 *  `update_duration_seconds`  Duration of update runs in seconds. (needs a start file)

Additionally two HTTP endpoints report healthiness and liveness depending
of the age of the end file.

# TODO

Use file watcher instead of static interval for updating.

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

