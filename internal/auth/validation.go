package auth

import (
	"errors"
	"regexp"
	"strings"

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
					return "backend.auth.username_too_short"
				case "max":
					return "backend.auth.username_too_long"
				case "required":
					return "backend.auth.username_required"
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

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{2,49}$`)

// ValidateNewUsername validates a username for new user creation.
// This is only applied to NEW usernames during registration/creation,
// not to existing usernames during login or updates.
func ValidateNewUsername(username string) error {
	username = strings.TrimSpace(username)
	
	if len(username) < 3 {
		return errors.New("backend.auth.username_too_short")
	}
	if len(username) > 50 {
		return errors.New("backend.auth.username_too_long")
	}
	
	if !usernameRegex.MatchString(username) {
		return errors.New("backend.auth.username_invalid_format")
	}
	
	return nil
}
