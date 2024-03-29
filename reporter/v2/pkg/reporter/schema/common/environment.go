// Copyright 2021 IBM Corp.
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

package common

const (
	ReportProductionEnv ReportEnvironment = "production"
	ReportSandboxEnv    ReportEnvironment = "stage"
)

type ReportEnvironment string

func (m ReportEnvironment) MarshalText() ([]byte, error) {
	return []byte(string(m)), nil
}

func (m *ReportEnvironment) UnmarshalText(data []byte) error {
	str := ReportEnvironment(string(data))
	*m = str
	return nil
}

func (m ReportEnvironment) String() string {
	return string(m)
}
