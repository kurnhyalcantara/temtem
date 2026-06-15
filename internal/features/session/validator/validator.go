// Package validator validates session feature inputs and converts failures
// into apperror.CodeInvalidArgument errors.
package validator

import (
	"github.com/kurnhyalcantara/temtem/internal/features/session/dto"
	"github.com/kurnhyalcantara/temtem/pkg/apperror"
	platvalidator "github.com/kurnhyalcantara/temtem/platform/validator"
)

type Validator struct {
	v *platvalidator.Validator
}

func New(v *platvalidator.Validator) *Validator {
	return &Validator{v: v}
}

func (val *Validator) CreateSession(in dto.CreateSessionInput) error {
	return val.check(in)
}

func (val *Validator) GetSession(in dto.GetSessionInput) error {
	return val.check(in)
}

func (val *Validator) RefreshSession(in dto.RefreshSessionInput) error {
	return val.check(in)
}

func (val *Validator) RevokeSession(in dto.RevokeSessionInput) error {
	return val.check(in)
}

func (val *Validator) check(in any) error {
	if err := val.v.Struct(in); err != nil {
		return apperror.Invalid(err.Error())
	}
	return nil
}
