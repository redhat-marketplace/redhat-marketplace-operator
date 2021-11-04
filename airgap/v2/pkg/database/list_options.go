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
	"strconv"

	"gorm.io/gorm"
)

type ListOption interface {
	ApplyToList(*ListOptions)
}

type ListOptions struct {
	Pagination  ListPagination
	ShowDeleted bool
}

func (o *ListOptions) ApplyOptions(opts []ListOption) *ListOptions {
	if o == nil {
		return o
	}

	//defaults
	o.Pagination = ListPagination{Page: 1, PageSize: 10}

	for _, opt := range opts {
		opt.ApplyToList(o)
	}

	return o
}

func (o *ListOptions) scopes() []func(db *gorm.DB) *gorm.DB {
	scopesToApply := []func(db *gorm.DB) *gorm.DB{}
	for _, scope := range listScopes {
		scopesToApply = append(scopesToApply, scope.ToScope(o))
	}

	return scopesToApply
}

func Paginate(pageToken string, pageSize int) ListOption {
	var (
		page int
		err  error
	)

	if pageToken == "" {
		page = 1
	} else {
		page, err = strconv.Atoi(pageToken)

		if err != nil {
			page = 1
		}
	}

	switch {
	case pageSize > 100:
		pageSize = 100
	case pageSize <= 0:
		pageSize = 10
	}

	return ListPagination{
		Page:     page,
		PageSize: pageSize,
	}
}

type ListPagination struct {
	Page, PageSize int
}

func (l ListPagination) ApplyToList(opts *ListOptions) {
	opts.Pagination = l
}

func ShowDeleted() ListOption {
	return showDeleted{}
}

type showDeleted struct{}

func (f showDeleted) ApplyToList(opts *ListOptions) {
	opts.ShowDeleted = true
}
