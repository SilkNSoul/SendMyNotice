package apierrors

import "fmt"

// UserError is an error intended to be displayed to the end user.
type UserError struct {
	Code        string // The internal/Lob code (e.g. "failed_deliverability_strictness")
	DevMessage  string // The technical log message
	UserMessage string // The friendly "fix it" message
}

func (e *UserError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.DevMessage)
}

// MapLobError translates a raw Lob error code into a UserError.
// Source: https://docs.lob.com/#errors
func MapLobError(code string, originalMsg string) *UserError {
	switch code {
	case "failed_deliverability_strictness":
		return &UserError{
			Code:        code,
			DevMessage:  originalMsg,
			UserMessage: "We could not verify this address exists. Please double-check the street number and spelling.",
		}
	case "invalid_address":
		return &UserError{
			Code:        code,
			DevMessage:  originalMsg,
			UserMessage: "The address format is incorrect. Please ensure you have a valid City, State, and Zip.",
		}
	case "address_length_exceeds_limit":
		return &UserError{
			Code:        code,
			DevMessage:  originalMsg,
			UserMessage: "The address line is too long (max 40 chars). Please abbreviate (e.g., 'St' instead of 'Street').",
		}
	case "rate_limit_exceeded":
		return &UserError{
			Code:        code,
			DevMessage:  originalMsg,
			UserMessage: "We are sending too many requests at once. Please wait a moment and try again.",
		}
	default:
		// Fallback for unknown 422 errors
		return &UserError{
			Code:        "unknown_validation_error",
			DevMessage:  originalMsg,
			UserMessage: "The mail carrier rejected this request. Please verify the information is correct.",
		}
	}
}