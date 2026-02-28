package gogram

import (
	"errors"
	"net/http"
	"time"
)

// Error represents a generic gogram error.
type Error struct {
	Code int
	Text string
}

func (e *Error) Error() string {
	return e.Text
}

// APIError represents a Telegram API error with a description.
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

// RetryError represents a rate limit error.
// It contains the duration to wait before retrying the request.
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

// MigrateError represents a chat migration error.
// It indicates that the group has been migrated to a supergroup with a new identifier.
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

// NewError creates a new Error with the given code and text.
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

// Base API errors returned by Telegram.
var (
	// ErrBadRequest indicates an invalid request payload or parameters.
	ErrBadRequest = NewError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	// ErrUnauthorized indicates missing or invalid authentication.
	ErrUnauthorized = NewError(http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
	// ErrForbidden indicates that the operation is not allowed.
	ErrForbidden = NewError(http.StatusForbidden, http.StatusText(http.StatusForbidden))
	// ErrNotFound indicates that the requested resource does not exist.
	ErrNotFound = NewError(http.StatusNotFound, http.StatusText(http.StatusNotFound))
	// ErrConflict indicates a conflicting request state.
	ErrConflict = NewError(http.StatusConflict, http.StatusText(http.StatusConflict))
	// ErrTooManyRequests indicates that rate limits were exceeded.
	ErrTooManyRequests = NewError(http.StatusTooManyRequests, http.StatusText(http.StatusTooManyRequests))
	// ErrInternalServerError indicates an internal Telegram server error.
	ErrInternalServerError = NewError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	// ErrBadGateway indicates a Telegram upstream gateway failure.
	ErrBadGateway = NewError(http.StatusBadGateway, http.StatusText(http.StatusBadGateway))
	// ErrServiceUnavailable indicates temporary Telegram service unavailability.
	ErrServiceUnavailable = NewError(http.StatusServiceUnavailable, http.StatusText(http.StatusServiceUnavailable))
	// ErrGatewayTimeout indicates a Telegram upstream timeout.
	ErrGatewayTimeout = NewError(http.StatusGatewayTimeout, http.StatusText(http.StatusGatewayTimeout))

	// ErrEOF indicates an empty response body where JSON was expected.
	ErrEOF = NewError(http.StatusBadRequest, "EOF")
)

// Known custom bad request errors returned by Telegram.
//
//nolint:lll
var (
	ErrBadRequestWrongRemoteFileIdentifierSpecified = NewError(http.StatusBadRequest, "Bad Request: wrong remote file identifier specified: Wrong character in the string")
	ErrBadRequestCantUseFileOfTypeDocumentAsPhoto   = NewError(http.StatusBadRequest, "Bad Request: can't use file of type Document as Photo")
	// ErrBadRequestParticipantIDInvalid indicates the bot cannot access this user yet.
	ErrBadRequestParticipantIDInvalid = NewError(http.StatusBadRequest, "Bad Request: PARTICIPANT_ID_INVALID")
	// ErrBadRequestChatNotFound indicates that the target chat does not exist or is inaccessible.
	ErrBadRequestChatNotFound = NewError(http.StatusBadRequest, "Bad Request: chat not found")
	// ErrBadRequestFileMustBeNonEmpty indicates that an uploaded file is empty.
	ErrBadRequestFileMustBeNonEmpty = NewError(http.StatusBadRequest, "Bad Request: file must be non-empty")
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

	case ErrBadRequestFileMustBeNonEmpty.Error():
		return ErrBadRequestFileMustBeNonEmpty

	default:
		return nil
	}
}

// Forbidden errors returned by Telegram.
var (
	// ErrForbiddenBotWasBlockedByTheUser indicates that the user blocked the bot.
	ErrForbiddenBotWasBlockedByTheUser = NewError(http.StatusForbidden, "Forbidden: bot was blocked by the user")
)

// Not-found flavored errors returned by Telegram.
var (
	// ErrNotFoundBanned indicates that the bot has been banned.
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

// Conflict errors returned by Telegram.
//
//nolint:lll
var (
	// ErrConflictWithBot indicates another getUpdates consumer is already running.
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
