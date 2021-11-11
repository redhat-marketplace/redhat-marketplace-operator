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

package database

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"emperror.dev/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/dataservice/v1/fileserver"
	modelsv2 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type filter struct {
	Filters fileserver.Filters
}

func (file filter) ToScope(opts *ListOptions) func(db *gorm.DB) *gorm.DB {
	if len(opts.Filters) == 0 {
		return func(db *gorm.DB) *gorm.DB {
			return db
		}
	}

	builder := &strings.Builder{}
	variables := []interface{}{}
	var filterError error

	for _, f := range opts.Filters {
		op := operatorToSql(f.Operator)
		left, err := strconv.Unquote(f.Left)
		if err != nil {
			left = f.Left
		}
		left = strings.Title(left)

		// fieldType used in left side
		fieldName, fieldType, err := getFieldMapping(left)

		if err != nil {
			filterError = err
			break
		}

		builder.WriteString(fmt.Sprintf("%s %s ?", fieldName, op))

		// write boolean operator if it is prsent
		if f.NextFilterOperator != fileserver.FilterBooleanOperator("") {
			builder.WriteString(" ")
			builder.WriteString(boolOperatorToSQL(f.NextFilterOperator))
			builder.WriteString(" ")
		}

		// set variable and append to array, unquote it in case it's quoted
		right, err := strconv.Unquote(f.Right)
		if err != nil {
			right = f.Right
		}

		if fieldType == reflect.TypeOf(time.Time{}) {
			rightAsDate, err1 := time.Parse(time.RFC3339, right)

			if err1 == nil {
				variables = append(variables, rightAsDate)
				continue
			}

			rightAsInt, err2 := strconv.ParseInt(right, 0, 64)

			if err2 == nil {
				variables = append(variables, time.Unix(rightAsInt, 0))
				continue
			}

			filterError = errors.Combine(err1, err2)
			break
		}

		variables = append(variables, right)
	}

	return func(db *gorm.DB) *gorm.DB {
		if filterError != nil {
			db.AddError(filterError)
		}

		return db.Where(builder.String(), variables...)
	}
}

func isDateField(in string) bool {
	return strings.Contains(in, "updated_at") ||
		strings.Contains(in, "deleted_at") ||
		strings.Contains(in, "created_at")
}

func camelToSnakeCase(in string) string {
	out := &strings.Builder{}
	for _, c := range in {
		if unicode.IsUpper(c) {
			out.WriteRune('_')
			out.WriteRune(unicode.ToLower(c))
			continue
		}
		out.WriteRune(c)
	}
	return out.String()
}

func boolOperatorToSQL(operator fileserver.FilterBooleanOperator) string {
	switch operator {
	case fileserver.FilterAnd:
		return "AND"
	case fileserver.FilterOr:
		return "OR"
	}

	return ""
}

func operatorToSql(operator fileserver.FilterOperator) string {
	switch operator {
	case fileserver.FilterEqual:
		return "="
	case fileserver.FilterNotEqual:
		return "<>"
	default:
		return operator.String()
	}
}

var (
	namingStrat = schema.NamingStrategy{}
	file, _     = schema.Parse(&modelsv2.StoredFile{}, &sync.Map{}, namingStrat)
	metadata, _ = schema.Parse(&modelsv2.StoredFileMetadata{}, &sync.Map{}, namingStrat)
	content, _  = schema.Parse(&modelsv2.StoredFileContent{}, &sync.Map{}, namingStrat)
)

func getFieldMapping(name string) (string, reflect.Type, error) {
	for _, schema := range []*schema.Schema{file, content, metadata} {
		f, ok := schema.FieldsByName[name]
		if ok {
			name := getFieldName(*schema, *f)
			return name, f.FieldType, nil
		}
	}

	return "", nil, errors.Errorf("field %s not found", name)
}

func getFieldName(schema schema.Schema, field schema.Field) string {
	tableName := namingStrat.TableName(schema.Name)
	fieldName := namingStrat.ColumnName(tableName, field.Name)
	return fmt.Sprintf("`%s`.`%s`", tableName, fieldName)
}
