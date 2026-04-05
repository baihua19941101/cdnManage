package provider

import "fmt"

type Service string

const (
	ServiceObjectStorage Service = "object_storage"
	ServiceCDN           Service = "cdn"
)

type ErrorCode string

const (
	ErrCodeUnsupportedProvider ErrorCode = "unsupported_provider"
	ErrCodeInvalidCredentials  ErrorCode = "invalid_credentials"
	ErrCodeConnectionFailed    ErrorCode = "connection_failed"
	ErrCodeTimeout             ErrorCode = "timeout"
	ErrCodeRateLimited         ErrorCode = "rate_limited"
	ErrCodeNotFound            ErrorCode = "not_found"
	ErrCodeInvalidRequest      ErrorCode = "invalid_request"
	ErrCodeOperationFailed     ErrorCode = "operation_failed"
)

type Error struct {
	ProviderType Type
	Service      Service
	Operation    string
	Code         ErrorCode
	Message      string
	Retryable    bool
	Cause        error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}

	scope := string(e.Service)
	if scope == "" {
		scope = "provider"
	}
	if e.Operation == "" {
		return fmt.Sprintf("%s %s: %s", e.ProviderType, scope, e.Message)
	}

	return fmt.Sprintf("%s %s %s: %s", e.ProviderType, scope, e.Operation, e.Message)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NewError(providerType Type, service Service, operation string, code ErrorCode, message string, retryable bool, cause error) *Error {
	return &Error{
		ProviderType: providerType,
		Service:      service,
		Operation:    operation,
		Code:         code,
		Message:      message,
		Retryable:    retryable,
		Cause:        cause,
	}
}
