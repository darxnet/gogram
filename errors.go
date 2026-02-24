package gogram

import (
	"errors"
	"net/http"
	"time"
)

type Error struct {
	Code int
	Text string
}

func (e *Error) Error() string {
	return e.Text
}

// APIError
// with description.
type APIError struct {
	Err         error
	Description string
}

func (e *APIError) Error() string {
	if e.Description == "" {
		return e.Err.Error()
	}

	return e.Description
}

func (e *APIError) Unwrap() error {
	return e.Err
}

// RetryError
// in case of exceeding flood control, the number of seconds left to wait before the request can be repeated.
type RetryError struct {
	Err        error
	RetryAfter time.Duration
}

func (e *RetryError) Error() string {
	return e.Err.Error()
}

func (e *RetryError) Unwrap() error {
	return e.Err
}

// MigrateError
// means the group has been migrated to a supergroup with the specified identifier.
type MigrateError struct {
	Err             error
	MigrateToChatID int64
}

func (e *MigrateError) Error() string {
	return e.Err.Error()
}

func (e *MigrateError) Unwrap() error {
	return e.Err
}

func NewError(code int, text string) error {
	return &Error{Code: code, Text: text}
}

func genError(code int, text, description string, params *ResponseParameters) error {
	switch code {
	case http.StatusBadRequest:
		return &APIError{
			Err:         errors.Join(ErrBadRequest, genErrorBadRequest(description)),
			Description: description,
		}

	case http.StatusUnauthorized:
		return &APIError{
			Err:         ErrUnauthorized,
			Description: description,
		}

	case http.StatusForbidden:
		return &APIError{
			Err:         ErrForbidden,
			Description: description,
		}

	case http.StatusNotFound:
		return &APIError{
			Err:         errors.Join(ErrNotFound, genErrorNotFound(description)),
			Description: description,
		}

	case http.StatusConflict:
		return &APIError{
			Err:         errors.Join(ErrConflict, genErrorConflict(description)),
			Description: description,
		}

	case http.StatusTooManyRequests:
		var retryAfter time.Duration

		if params != nil && params.RetryAfter != 0 {
			retryAfter = time.Duration(params.RetryAfter) * time.Second
		} else {
			retryAfter = defaultTimeout
		}

		return &RetryError{
			Err:        ErrTooManyRequests,
			RetryAfter: retryAfter,
		}

	case http.StatusInternalServerError:
		return &RetryError{
			Err:        ErrInternalServerError,
			RetryAfter: defaultTimeout,
		}

	case http.StatusBadGateway:
		return &RetryError{
			Err:        ErrBadGateway,
			RetryAfter: defaultTimeout,
		}

	case http.StatusServiceUnavailable:
		return &RetryError{
			Err:        ErrServiceUnavailable,
			RetryAfter: defaultTimeout,
		}

	case http.StatusGatewayTimeout:
		return &RetryError{
			Err:        ErrGatewayTimeout,
			RetryAfter: defaultTimeout,
		}

	default:
		return &APIError{
			Err:         &Error{Code: code, Text: text},
			Description: description,
		}
	}
}

var (
	ErrBadRequest          = NewError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	ErrUnauthorized        = NewError(http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
	ErrForbidden           = NewError(http.StatusForbidden, http.StatusText(http.StatusForbidden))
	ErrNotFound            = NewError(http.StatusNotFound, http.StatusText(http.StatusNotFound))
	ErrConflict            = NewError(http.StatusConflict, http.StatusText(http.StatusConflict))
	ErrTooManyRequests     = NewError(http.StatusTooManyRequests, http.StatusText(http.StatusTooManyRequests))
	ErrInternalServerError = NewError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	ErrBadGateway          = NewError(http.StatusBadGateway, http.StatusText(http.StatusBadGateway))
	ErrServiceUnavailable  = NewError(http.StatusServiceUnavailable, http.StatusText(http.StatusServiceUnavailable))
	ErrGatewayTimeout      = NewError(http.StatusGatewayTimeout, http.StatusText(http.StatusGatewayTimeout))

	ErrEOF = NewError(http.StatusBadRequest, "EOF")
)

//nolint:lll
var (
	ErrBadRequestWrongRemoteFileIdentifierSpecified = NewError(http.StatusBadRequest, "Bad Request: wrong remote file identifier specified: Wrong character in the string")
	ErrBadRequestCantUseFileOfTypeDocumentAsPhoto   = NewError(http.StatusBadRequest, "Bad Request: can't use file of type Document as Photo")
	// ErrBadRequestParticipantIDInvalid means what bot don`t meet this user yet (and can`t use his id in any request).
	ErrBadRequestParticipantIDInvalid = NewError(http.StatusBadRequest, "Bad Request: PARTICIPANT_ID_INVALID")
	ErrBadRequestChatNotFound         = NewError(http.StatusBadRequest, "Bad Request: chat not found")
	ErrBadRequestFileMustBeNonEmpty   = NewError(http.StatusBadRequest, "Bad Request: file must be non-empty")
)

func genErrorBadRequest(description string) error {
	switch description {
	case ErrBadRequestWrongRemoteFileIdentifierSpecified.Error():
		return ErrBadRequestWrongRemoteFileIdentifierSpecified

	case ErrBadRequestCantUseFileOfTypeDocumentAsPhoto.Error():
		return ErrBadRequestCantUseFileOfTypeDocumentAsPhoto

	case ErrBadRequestParticipantIDInvalid.Error():
		return ErrBadRequestParticipantIDInvalid

	case ErrBadRequestChatNotFound.Error():
		return ErrBadRequestChatNotFound

	default:
		return nil
	}
}

var (
	ErrForbiddenBotWasBlockedByTheUser = NewError(http.StatusForbidden, "Forbidden: bot was blocked by the user")
)

var (
	ErrNotFoundBanned = NewError(http.StatusNotFound, "Contact https://t.me/BotSupport for assistance")
)

func genErrorNotFound(description string) error {
	switch description {
	case ErrNotFoundBanned.Error():
		return ErrNotFoundBanned

	default:
		return nil
	}
}

//nolint:lll
var (
	ErrConflictWithBot = NewError(http.StatusConflict, "Conflict: terminated by other getUpdates request; make sure that only one bot instance is running")
)

func genErrorConflict(description string) error {
	switch description {
	case ErrConflictWithBot.Error():
		return ErrConflictWithBot

	default:
		return nil
	}
}
