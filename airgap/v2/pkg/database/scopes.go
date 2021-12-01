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
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	listScopes = []listOptionsToScope{paginator{}, showHidden{}, filter{}}
)

type listOptionsToScope interface {
	ToScope(opts *ListOptions) func(db *gorm.DB) *gorm.DB
}

type paginator struct{}

func (p paginator) ToScope(opts *ListOptions) func(db *gorm.DB) *gorm.DB {
	if opts.Pagination == nil {
		return func(db *gorm.DB) *gorm.DB {
			return db
		}
	}

	return func(db *gorm.DB) *gorm.DB {
		page, pageSize := opts.Pagination.Page, opts.Pagination.PageSize

		if page == 0 {
			page = 1
		}

		switch {
		case pageSize > 100:
			pageSize = 100
		case pageSize <= 0:
			pageSize = 10
		}

		offset := (page - 1) * pageSize
		return db.Offset(offset).Limit(pageSize + 1)
	}
}

type showHidden struct{}

func (s showHidden) ToScope(opts *ListOptions) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if opts.ShowDeleted {
			return db.Unscoped().Preload(clause.Associations, func(db *gorm.DB) *gorm.DB {
				return db.Unscoped()
			})
		}

		return db
	}
}
