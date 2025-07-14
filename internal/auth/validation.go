package auth

import (
	"errors"

	"github.com/go-playground/validator/v10"
)

// validationErrorMessage returns a user-friendly validation error message.
func validationErrorMessage(err error) string {
	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) {
		for _, ve := range verrs {
			switch ve.Field() {
			case "Password", "NewPassword":
				switch ve.Tag() {
				case "min":
					return "Password must be at least 8 characters long"
				case "required":
					return "Password is required"
				}
			}
		}
	}
	return "Invalid request"
}
