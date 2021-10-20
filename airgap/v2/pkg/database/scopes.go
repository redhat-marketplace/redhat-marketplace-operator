package database

import (
	"gorm.io/gorm"
)

var (
	listScopes = []listOptionsToScope{paginator{}, showHidden{}}
)

type listOptionsToScope interface {
	ToScope(opts *ListOptions) func(db *gorm.DB) *gorm.DB
}

type paginator struct{}

func (p paginator) ToScope(opts *ListOptions) func(db *gorm.DB) *gorm.DB {
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
		return db.Offset(offset).Limit(pageSize)
	}
}

type showHidden struct{}

func (s showHidden) ToScope(opts *ListOptions) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if opts.ShowDeleted {
			return db.Unscoped()
		}

		return db
	}
}
