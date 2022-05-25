package rectest

// Code generated by github.com/launchdarkly/go-options.  DO NOT EDIT.

import "fmt"

import (
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

import "github.com/google/go-cmp/cmp"

type ApplyStepOptionFunc func(c *stepOptions) error

func (f ApplyStepOptionFunc) apply(c *stepOptions) error {
	return f(c)
}

func newStepOptions(options ...StepOption) (stepOptions, error) {
	var c stepOptions
	err := applyStepOptionsOptions(&c, options...)
	return c, err
}

func applyStepOptionsOptions(c *stepOptions, options ...StepOption) error {
	for _, o := range options {
		if err := o.apply(c); err != nil {
			return err
		}
	}
	return nil
}

type StepOption interface {
	apply(*stepOptions) error
}

type withStepNameImpl struct {
	o string
}

func (o withStepNameImpl) apply(c *stepOptions) error {
	c.StepName = o.o
	return nil
}

func (o withStepNameImpl) Equal(v withStepNameImpl) bool {
	switch {
	case !cmp.Equal(o.o, v.o):
		return false
	}
	return true
}

func (o withStepNameImpl) String() string {
	name := "WithStepName"

	// hack to avoid go vet error about passing a function to Sprintf
	var value interface{} = o.o
	return fmt.Sprintf("%s: %+v", name, value)
}

func WithStepName(o string) StepOption {
	return withStepNameImpl{
		o: o,
	}
}

type withRequestImpl struct {
	o reconcile.Request
}

func (o withRequestImpl) apply(c *stepOptions) error {
	c.Request = o.o
	return nil
}

func (o withRequestImpl) Equal(v withRequestImpl) bool {
	switch {
	case !cmp.Equal(o.o, v.o):
		return false
	}
	return true
}

func (o withRequestImpl) String() string {
	name := "WithRequest"

	// hack to avoid go vet error about passing a function to Sprintf
	var value interface{} = o.o
	return fmt.Sprintf("%s: %+v", name, value)
}

func WithRequest(o reconcile.Request) StepOption {
	return withRequestImpl{
		o: o,
	}
}