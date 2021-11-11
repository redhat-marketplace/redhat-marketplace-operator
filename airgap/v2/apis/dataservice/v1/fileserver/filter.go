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

package fileserver

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/scanner"
	"unicode"

	"emperror.dev/errors"
)

// a==b,a>b,a<b
// ==,>=,<=,!==
//
type FilterOperator string

const (
	FilterEqual            FilterOperator = "=="
	FilterNotEqual         FilterOperator = "!="
	FilterGreaterThan      FilterOperator = ">"
	FilterGreaterThanEqual FilterOperator = ">="
	FilterLessThan         FilterOperator = "<"
	FilterLessThanEqual    FilterOperator = "<="
)

func (f FilterOperator) String() string {
	return string(f)
}

type Filter struct {
	Left               string
	Operator           FilterOperator
	Right              string
	NextFilterOperator FilterBooleanOperator
}

var (
	ErrParseFailure = errors.Sentinel("filter parse failure")
)

func (f *Filter) MarshalText() (text []byte, err error) {
	return []byte(fmt.Sprintf("%s%s%s%s", f.Left, string(f.Operator), f.Right, string(f.NextFilterOperator))), nil
}

type FilterBooleanOperator string

const (
	FilterAnd FilterBooleanOperator = "&&"
	FilterOr  FilterBooleanOperator = "||"
)

type Filters []*Filter

func (f Filters) MarshalText() (text []byte, err error) {
	buf := &bytes.Buffer{}
	for _, filter := range f {
		text, err = filter.MarshalText()

		if err != nil {
			return []byte{}, err
		}

		buf.Write(text)
	}
	return buf.Bytes(), nil
}

const ErrFilterBadFormat = errors.Sentinel("incorrect filter should follow the form '[arg] [op] [arg] ([boolop] [arg] [op] [arg])...'")

func (f *Filters) UnmarshalText(text []byte) error {
	tok := &filterTokenizer{}
	tok.initTokenizer(string(text))

	filter := &Filter{}
	tokens := []interface{}{}
	for {
		token, err := tok.nextToken()
		if err == io.EOF {
			if filter != nil {
				*f = append(*f, filter)
				tokens = nil
				filter = nil
			}
			if len(tokens) != 0 {
				return errors.Wrap(ErrFilterBadFormat, "incomplete filter")
			}
			break
		}

		tokens = append(tokens, token)

		if len(tokens) == 3 {
			left, ok := tokens[0].(string)

			if !ok {
				return errors.WrapIff(ErrFilterBadFormat, "left token is not a string %v %t", tokens[0], tokens[0])
			}

			op, ok := tokens[1].(FilterOperator)

			if !ok {
				return errors.WrapIff(ErrFilterBadFormat, "operator token is not an operator %v %t", tokens[1], tokens[1])
			}

			right, ok := tokens[2].(string)

			if !ok {
				return errors.WrapIff(ErrFilterBadFormat, "right token is not a string %v %t", tokens[2], tokens[2])
			}

			filter = &Filter{
				Left:     left,
				Operator: op,
				Right:    right,
			}
		}

		if len(tokens) == 4 {
			boolOperator, ok := tokens[3].(FilterBooleanOperator)

			if !ok {
				return errors.WrapIff(ErrFilterBadFormat, "boolean token is not a string %v %t", tokens[3], tokens[3])
			}

			filter.NextFilterOperator = boolOperator
			tokens = []interface{}{}
			*f = append(*f, filter)
			filter = nil
		}
	}

	return nil
}

type filterTokenizer struct {
	s *scanner.Scanner
}

func (t *filterTokenizer) initTokenizer(in string) {
	var s scanner.Scanner
	s.Init(strings.NewReader(string(in)))
	s.Filename = "example"
	t.s = &s
}

func (t *filterTokenizer) nextToken() (interface{}, error) {
	s := t.s
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		switch tok {
		case '=':
			if s.Peek() == '=' {
				s.Next()
				return FilterEqual, nil
			}

			return nil, errors.New("unknown symbol")
		case '!':
			if s.Peek() == '=' {
				s.Next()
				return FilterNotEqual, nil
			}

			return nil, errors.New("unknown symbol")
		case '<':
			if s.Peek() == '=' {
				s.Next()
				return FilterLessThanEqual, nil
			}
			return FilterLessThan, nil
		case '>':
			if s.Peek() == '=' {
				s.Next()
				return FilterGreaterThanEqual, nil
			}
			return FilterGreaterThan, nil
		case '&':
			if s.Peek() == '&' {
				s.Next()
				return FilterAnd, nil
			}
		case '|':
			if s.Peek() == '|' {
				s.Next()
				return FilterOr, nil
			}
		default:
			if s.Peek() != '.' {
				return s.TokenText(), nil
			}

			text := &strings.Builder{}
			text.WriteString(s.TokenText())

		forLoop:
			for {
				peek := s.Peek()
				switch {
				case unicode.IsLetter(peek):
					s.Scan()
					text.WriteString(s.TokenText())
				case unicode.IsDigit(peek):
					s.Scan()
					text.WriteString(s.TokenText())
				case peek == '_' || peek == '.':
					s.Scan()
					text.WriteString(s.TokenText())
				default:
					break forLoop
				}
			}
			return text.String(), nil
		}
	}

	return "", io.EOF
}

func (t *filterTokenizer) position() int {
	return t.s.Offset
}

func ParseFilters(in string) (Filters, error) {

	return Filters{}, nil
}
