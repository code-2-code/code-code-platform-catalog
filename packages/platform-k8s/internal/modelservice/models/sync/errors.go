package sync

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

func definitionSourceEndpointUnavailable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var target *url.Error
	if errors.As(err, &target) {
		if target.Timeout() {
			return true
		}
		if definitionSourceEndpointUnavailable(target.Err) {
			return true
		}
	}

	normalized := strings.ToLower(strings.TrimSpace(err.Error()))
	if normalized == "" {
		return false
	}
	return strings.Contains(normalized, "connection reset by peer") ||
		strings.Contains(normalized, "connect: connection refused") ||
		strings.Contains(normalized, "tls handshake timeout") ||
		strings.Contains(normalized, "context deadline exceeded") ||
		strings.Contains(normalized, "client.timeout exceeded") ||
		strings.Contains(normalized, "i/o timeout") ||
		strings.Contains(normalized, "no such host") ||
		strings.HasSuffix(normalized, ": eof") ||
		normalized == "eof"
}
