package main

import (
	"fmt"
)

type Validator struct {
	Errors []string
}

func (v *Validator) Error(err string) {
	v.Errors = append(v.Errors, err)
}

func (v *Validator) Errorf(format string, a ...interface{}) {
	v.Error(fmt.Sprintf(format, a...))
}
