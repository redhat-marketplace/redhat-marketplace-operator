package database

import "gorm.io/gorm"

type ListOption interface {
	ApplyToList(*ListOptions)
}

type ListOptions struct {
	Pagination  ListPagination
	ShowDeleted bool
}

func (o *ListOptions) ApplyOptions(opts []ListOption) *ListOptions {
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

func Paginate(page, pageSize int) ListOption {
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
