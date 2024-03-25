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
	"emperror.dev/errors"

	kazaam "github.com/qntfy/kazaam/v4"
)

type Transformer struct {
	transformerType   string
	transformerText   string
	kazaamTransformer *kazaam.Kazaam
}

func (t *Transformer) Transform(json []byte) ([]byte, error) {
	switch t.transformerType {
	case "kazaam":
		kazaamOut, err := t.kazaamTransformer.Transform(json)
		return kazaamOut, err
	default:
		return nil, errors.New("unsupported transformer type")
	}
}

func NewTransformer(transformerType string, transformerText string) (Transformer, error) {
	switch transformerType {
	case "kazaam":
		var k, err = kazaam.NewKazaam(transformerText)
		if err != nil {
			return Transformer{}, errors.Wrap(err, "invalid transformer string")
		} else {
			return Transformer{transformerType: transformerType, transformerText: transformerText, kazaamTransformer: k}, nil
		}
	}
	return Transformer{}, errors.New("unsupported transformer type")
}
