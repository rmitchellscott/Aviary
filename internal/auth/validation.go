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
			case "Email":
				switch ve.Tag() {
				case "email":
					return "Please enter a valid email address"
				case "required":
					return "Email is required"
				}
			case "Username":
				switch ve.Tag() {
				case "min":
					return "Username must be at least 3 characters long"
				case "max":
					return "Username must be no more than 50 characters long"
				case "required":
					return "Username is required"
				}
			case "CurrentPassword":
				switch ve.Tag() {
				case "required":
					return "Current password is required"
				}
			}
		}
	}
	return "Invalid request"
}
