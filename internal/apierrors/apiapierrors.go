package apierrors

import "fmt"

type UserError struct {
	Code        string
	DevMessage  string 
	UserMessage string 
}

func (e *UserError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.DevMessage)
}


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
		return &UserError{
			Code:        "unknown_validation_error",
			DevMessage:  originalMsg,
			UserMessage: "The mail carrier rejected this request. Please verify the information is correct.",
		}
	}
}