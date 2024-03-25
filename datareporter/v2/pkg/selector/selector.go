// Copyright 2024 IBM Corp.
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

package selector

import (
	"golang.org/x/exp/slices"

	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"

	"emperror.dev/errors"

	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/api/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/events"
)

type Selector interface {
	Matches(any) bool
}

type JsonPathsSelector []jp.Expr

// Parse string array of jsonpaths and return JsonPathsSelector for matching expressions
// Returns an error if a parse fails
func NewJsonPathsSelector(jsonpaths []string) (JsonPathsSelector, error) {
	var jps JsonPathsSelector
	for _, jsonpath := range jsonpaths {
		x, err := jp.ParseString(jsonpath)
		if err != nil {
			return jps, err
		}
		jps = append(jps, x)
	}
	return jps, nil
}

// Match json string against expressions in JsonPathsSelector
// Returns true if all jsonpath expressions produce a result (AND)
// Returns true if there are no expressions in the JsonPathsSelector
// Returns false if input does not parse as json
func (js JsonPathsSelector) Matches(jsonString string) bool {
	obj, err := oj.ParseString(jsonString)
	if err != nil {
		return false
	}

	for _, x := range js {
		result := x.Get(obj)
		if len(result) == 0 {
			return false
		}
	}

	return true
}

type UsersSelector []string

// Return true if UsersSelector contains entry
// Return true if UsersSelector is empty
func (us UsersSelector) Matches(user string) bool {
	if len(us) == 0 {
		return true
	}
	return slices.Contains(us, user)
}

type JsonUser struct {
	JsonString string
	UserString string
}

type DataFilterSelector struct {
	JsonPathsSelector
	UsersSelector
}

func NewDataFilterSelector(sel v1alpha1.Selector) (DataFilterSelector, error) {
	jps, err := NewJsonPathsSelector(sel.MatchExpressions)
	if err != nil {
		return DataFilterSelector{}, errors.Wrap(err, "failed to parse datafilter selector matchexpression")
	}
	return DataFilterSelector{JsonPathsSelector: jps, UsersSelector: sel.MatchUsers}, nil
}

// Returns true if
// Incoming json matches the JsonPathsSelector
// User matches the UsersSelector
func (dfs DataFilterSelector) Matches(event events.Event) bool {
	return dfs.JsonPathsSelector.Matches(string(event.RawMessage)) && dfs.UsersSelector.Matches(event.User)
}
