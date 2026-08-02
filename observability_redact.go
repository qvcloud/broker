package broker

import (
	"context"
	"errors"
	"regexp"
	"strings"
)

var sensitiveValuePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(password|passwd|token|secret|api[_-]?key|credential|signature|sig|x-amz-signature)=([^&\s]+)`),
	regexp.MustCompile(`(?i)(amqp|redis|nats|https?)://([^/@\s]+)@`),
	regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`),
}

func sanitizeSummary(value string) string {
	for _, pattern := range sensitiveValuePatterns {
		value = pattern.ReplaceAllString(value, "$1=[REDACTED]")
	}
	value = strings.TrimSpace(value)
	if len(value) > 512 {
		value = value[:512] + "…"
	}
	return value
}

func safeError(operation Operation, err error) *SafeError {
	if err == nil {
		return nil
	}
	category := errorCategory(operation, err)
	return &SafeError{Category: category, Summary: sanitizeSummary(err.Error())}
}

func safeErrorWithFields(operation Operation, err error, fields []string) *SafeError {
	safe := safeError(operation, err)
	if safe == nil {
		return nil
	}
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		pattern := regexp.MustCompile(`(?i)(` + regexp.QuoteMeta(field) + `)=([^&\s]+)`)
		safe.Summary = pattern.ReplaceAllString(safe.Summary, "$1=[REDACTED]")
	}
	return safe
}

func errorCategory(operation Operation, err error) ErrorCategory {
	if errors.Is(err, context.Canceled) {
		return ErrorCategoryCanceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrorCategoryTimeout
	}
	if errors.Is(err, ErrUnsupported) {
		return ErrorCategoryUnsupported
	}
	switch operation {
	case OperationConnect:
		return ErrorCategoryConnection
	case OperationPublish:
		return ErrorCategoryPublish
	case OperationReceive:
		return ErrorCategoryConsume
	case OperationHandler:
		return ErrorCategoryHandler
	case OperationAck:
		return ErrorCategoryAck
	case OperationNack:
		return ErrorCategoryNack
	case OperationReconnect:
		return ErrorCategoryReconnect
	case OperationDisconnect:
		return ErrorCategoryShutdown
	default:
		return ErrorCategoryUnknown
	}
}
