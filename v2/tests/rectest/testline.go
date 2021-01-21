// Copyright 2020 IBM Corp.
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

package rectest

import (
	"fmt"
	"io"
	"runtime"

	"github.com/pkg/errors"
)

// testLine is a helper to get the original line of the function
// call and create output to help test devs.
type testLine struct {
	msg   string
	stack errors.StackTrace
	err   error
}

func (t *testLine) Error() string {
	return t.msg
}

func NewTestLine(message string, up int) *testLine {
	pc := make([]uintptr, 1)
	n := runtime.Callers(up, pc)

	if n == 0 {
		return &testLine{msg: message}
	}

	trace := make(errors.StackTrace, len(pc))

	for i, ptr := range pc {
		trace[i] = errors.Frame(ptr)
	}

	return &testLine{msg: message, stack: trace}
}

func (t *testLine) TestLineError(err error) error {
	if err == nil {
		return nil
	}

	t.err = err
	return t
}

func (t *testLine) Unwrap() error { return t.err }

func (t *testLine) Cause() error { return t.err }

func (t *testLine) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "%+v", t.Cause())
			t.stack.Format(s, verb)
			return
		}
		fallthrough
	case 's':
		io.WriteString(s, t.Error())
	case 'q':
		fmt.Fprintf(s, "%q", t.Error())
	}
}
