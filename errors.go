package gogram

import (
	"errors"
	"net/http"
	"time"
)

const defaultTimeoutOnError = 5 * time.Second

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
		// "EOF" means Telegram hit the end of our request body early
		// (connection dropped mid-upload): transient, retried when the
		// body can be rewound.
		if description == errTextEOF {
			return &RetryError{Err: ErrEOF, RetryAfter: defaultTimeoutOnError}
		}

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
			retryAfter = defaultTimeoutOnError
		}

		return &RetryError{
			Err:        ErrTooManyRequests,
			RetryAfter: retryAfter,
		}

	case http.StatusInternalServerError:
		return &RetryError{
			Err:        ErrInternalServerError,
			RetryAfter: defaultTimeoutOnError,
		}

	case http.StatusBadGateway:
		return &RetryError{
			Err:        ErrBadGateway,
			RetryAfter: defaultTimeoutOnError,
		}

	case http.StatusServiceUnavailable:
		return &RetryError{
			Err:        ErrServiceUnavailable,
			RetryAfter: defaultTimeoutOnError,
		}

	case http.StatusGatewayTimeout:
		return &RetryError{
			Err:        ErrGatewayTimeout,
			RetryAfter: defaultTimeoutOnError,
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

	// ErrEOF indicates Telegram answered "EOF": a bare "EOF" or empty body,
	// or a 400 with description "EOF". The client wraps it in a [RetryError].
	ErrEOF = NewError(http.StatusBadRequest, errTextEOF)
)

// Error texts of known Telegram descriptions; constants so genError* can
// switch on them without method calls.
//
//nolint:lll
const (
	errTextWrongRemoteFileID    = "Bad Request: wrong remote file identifier specified: Wrong character in the string"
	errTextDocumentAsPhoto      = "Bad Request: can't use file of type Document as Photo"
	errTextParticipantIDInvalid = "Bad Request: PARTICIPANT_ID_INVALID"
	errTextChatNotFound         = "Bad Request: chat not found"
	errTextFileMustBeNonEmpty   = "Bad Request: file must be non-empty"
	errTextBotBlockedByUser     = "Forbidden: bot was blocked by the user"
	errTextBanned               = "Contact https://t.me/BotSupport for assistance"
	errTextConflictWithBot      = "Conflict: terminated by other getUpdates request; make sure that only one bot instance is running"
	errTextEOF                  = "EOF"
	errTextConflictWebhook      = "Conflict: can't use getUpdates method while webhook is active; use deleteWebhook to delete the webhook first"
)

// Known custom bad request errors returned by Telegram.
var (
	// ErrBadRequestWrongRemoteFileIdentifierSpecified indicates a malformed file_id.
	ErrBadRequestWrongRemoteFileIdentifierSpecified = NewError(http.StatusBadRequest, errTextWrongRemoteFileID)
	// ErrBadRequestCantUseFileOfTypeDocumentAsPhoto indicates a document file_id sent as a photo.
	ErrBadRequestCantUseFileOfTypeDocumentAsPhoto = NewError(http.StatusBadRequest, errTextDocumentAsPhoto)
	// ErrBadRequestParticipantIDInvalid indicates the bot cannot access this user yet.
	ErrBadRequestParticipantIDInvalid = NewError(http.StatusBadRequest, errTextParticipantIDInvalid)
	// ErrBadRequestChatNotFound indicates that the target chat does not exist or is inaccessible.
	ErrBadRequestChatNotFound = NewError(http.StatusBadRequest, errTextChatNotFound)
	// ErrBadRequestFileMustBeNonEmpty indicates that an uploaded file is empty.
	ErrBadRequestFileMustBeNonEmpty = NewError(http.StatusBadRequest, errTextFileMustBeNonEmpty)
)

func genErrorBadRequest(description string) error {
	switch description {
	case errTextWrongRemoteFileID:
		return ErrBadRequestWrongRemoteFileIdentifierSpecified

	case errTextDocumentAsPhoto:
		return ErrBadRequestCantUseFileOfTypeDocumentAsPhoto

	case errTextParticipantIDInvalid:
		return ErrBadRequestParticipantIDInvalid

	case errTextChatNotFound:
		return ErrBadRequestChatNotFound

	case errTextFileMustBeNonEmpty:
		return ErrBadRequestFileMustBeNonEmpty

	default:
		return nil
	}
}

// Forbidden errors returned by Telegram.
var (
	// ErrForbiddenBotWasBlockedByTheUser indicates that the user blocked the bot.
	ErrForbiddenBotWasBlockedByTheUser = NewError(http.StatusForbidden, errTextBotBlockedByUser)
)

// Not-found flavored errors returned by Telegram.
var (
	// ErrNotFoundBanned indicates that the bot has been banned.
	ErrNotFoundBanned = NewError(http.StatusNotFound, errTextBanned)
)

func genErrorNotFound(description string) error {
	switch description {
	case errTextBanned:
		return ErrNotFoundBanned

	default:
		return nil
	}
}

// Conflict errors returned by Telegram.
var (
	// ErrConflictWithBot indicates another getUpdates consumer is already running.
	ErrConflictWithBot = NewError(http.StatusConflict, errTextConflictWithBot)
	// ErrConflictWebhookActive indicates getUpdates was called while a webhook is set;
	// [Client.Start] deletes the webhook and retries on it.
	ErrConflictWebhookActive = NewError(http.StatusConflict, errTextConflictWebhook)
)

func genErrorConflict(description string) error {
	switch description {
	case errTextConflictWithBot:
		return ErrConflictWithBot

	case errTextConflictWebhook:
		return ErrConflictWebhookActive

	default:
		return nil
	}
}
