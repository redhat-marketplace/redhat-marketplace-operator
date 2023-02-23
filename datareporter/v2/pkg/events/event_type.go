// Copyright 2023 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package events

// config needed to provideDataServiceConfig
type Config struct {
	OutputDirectory      string
	DataServiceTokenFile string
	DataServiceCertFile  string
	Namespace            string
}

type Key string       // the api key the event was sent with
type EventJson []byte // the json which should be already validated with json.Valid()

type Event struct {
	Key
	EventJson
}
type Events []Event
type EventJsons []EventJson
