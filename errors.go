package onedrivefs

import "net/http"

// This is a list of some error codes returned by OneDrive API.
const (
	// AccessDeniedErrorCode means that the caller doesn't have permission to perform the action.
	AccessDeniedErrorCode = "accessDenied"
	// ActivityLimitReachedErrorCode means that the app or user has been throttled.
	ActivityLimitReachedErrorCode = "activityLimitReached"
	// GeneralExceptionErrorCode means that an unspecified error has occurred.
	GeneralExceptionErrorCode = "generalException"
	// InvalidRangeErrorCode means that the specified byte range is invalid or unavailable.
	InvalidRangeErrorCode = "invalidRange"
	// InvalidRequestErrorCode means that the request is malformed or incorrect.
	InvalidRequestErrorCode = "invalidRequest"
	// ItemNotFoundErrorCode means that the resource could not be found.
	ItemNotFoundErrorCode = "itemNotFound"
	// MalwareDetectedErrorCode means that malware was detected in the requested resource.
	MalwareDetectedErrorCode = "malwareDetected"
	// NameAlreadyExistsErrorCode means that the specified item name already exists.
	NameAlreadyExistsErrorCode = "nameAlreadyExists"
	// NotAllowedErrorCode means that the action is not allowed by the system.
	NotAllowedErrorCode = "notAllowed"
	// NotSupportedErrorCode means that the request is not supported by the system.
	NotSupportedErrorCode = "notSupported"
	// ResourceModifiedErrorCode means that the resource being updated has changed since the caller last read it, usually an eTag mismatch.
	ResourceModifiedErrorCode = "resourceModified"
	// ResyncRequiredErrorCode means that the delta token is no longer valid, and the app must reset the sync state.
	ResyncRequiredErrorCode = "resyncRequired"
	// ServiceNotAvailableErrorCode means that the service is not available. Try the request again after a delay. There may be a Retry-After header.
	ServiceNotAvailableErrorCode = "serviceNotAvailable"
	// QuotaLimitReachedErrorCode means that the user has reached their quota limit.
	QuotaLimitReachedErrorCode = "quotaLimitReached"
	// UnauthenticatedErrorCode means that the caller is not authenticated.
	UnauthenticatedErrorCode = "unauthenticated"
)

// OneDriveAPIError represents the error in the response returned by OneDrive drive API.
type OneDriveAPIError struct {
	Code             string      `json:"code"`
	Message          string      `json:"message"`
	LocalizedMessage string      `json:"localizedMessage"`
	InnerError       *InnerError `json:"innerError"`
	ResponseHeader   http.Header `json:"-"`
}

func (e *OneDriveAPIError) Error() string {
	if e.InnerError != nil {
		return e.Code + " - " + e.Message + " (" + e.InnerError.Date + ")"
	}
	return e.Code + " - " + e.Message
}

type InnerError struct {
	Date            string `json:"date"`
	RequestID       string `json:"request-id"`
	ClientRequestID string `json:"client-request-id"`
}
