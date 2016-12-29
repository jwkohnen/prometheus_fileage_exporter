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

package prometheus_fileage_exporter

import "time"

type Config struct {
	StartFile        string
	EndFile          string
	HostPort         string
	PromEndpoint     string
	HealthEndpoint   string
	LivenessEndpoint string
	HealthTimeout    time.Duration
	LivenessTimeout  time.Duration
	Welpenschutz     time.Duration
	DirectoryTimeout time.Duration
	Namespace        string
}
