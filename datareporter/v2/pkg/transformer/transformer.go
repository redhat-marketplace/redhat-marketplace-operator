// Copyright 2024 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package transformer

import (
	"errors"

	kazaam "github.com/qntfy/kazaam/v4"
)

type Transformer struct {
	transformerType string
	transformerText string
}

func (t *Transformer) Valid() bool {
	switch t.transformerType {
	case "kazaam":
		_, err := kazaam.NewKazaam(t.transformerText)
		if err == nil {
			return true
		}
	}
	return false
}

func (t *Transformer) Transform(json []byte) ([]byte, error) {
	switch t.transformerType {
	case "kazaam":
		k, err := kazaam.NewKazaam(t.transformerText)
		if err != nil {
			return nil, err
		}
		kazaamOut, _ := k.Transform(json)
		return kazaamOut, nil
	default:
		return nil, errors.New("unsupported transformer type")
	}
}

func NewTransformer(transformerType string, transformerText string) Transformer {
	return Transformer{transformerType: transformerType, transformerText: transformerText}
}
