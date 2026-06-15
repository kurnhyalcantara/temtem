// Package validator wraps go-playground/validator for struct validation.
package validator

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Validator validates structs using `validate` tags.
type Validator struct {
	validate *validator.Validate
}

func New() *Validator {
	return &Validator{validate: validator.New(validator.WithRequiredStructEnabled())}
}

// Struct validates v and returns a single human-readable error listing every
// failed field, or nil.
func (v *Validator) Struct(s any) error {
	err := v.validate.Struct(s)
	if err == nil {
		return nil
	}
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return err
	}
	msgs := make([]string, 0, len(verrs))
	for _, fe := range verrs {
		msgs = append(msgs, fmt.Sprintf("%s: failed %q validation", fe.Field(), fe.Tag()))
	}
	return errors.New(strings.Join(msgs, "; "))
}
