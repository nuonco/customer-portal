package nuon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
)

// IsUnreachable reports whether err means the Nuon API could not be reached at
// all — DNS failure, connection refused, TLS handshake failure, or a timeout —
// as opposed to the API answering and rejecting the request.
//
// Callers need to surface these very differently. An unreachable API is a
// configuration or infrastructure problem ("is NUON_API_URL right? is ctl-api
// running?"), whereas a rejection is a credentials problem. Reporting the former
// as "invalid API token" sends people looking in entirely the wrong place.
func IsUnreachable(err error) bool {
	if err == nil {
		return false
	}

	// A cancelled or timed-out request never got an answer we can trust.
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}

	// net.Error covers timeouts and most dial failures; *url.Error is what
	// net/http wraps transport failures in; *net.OpError and *net.DNSError are
	// the concrete causes underneath. go-swagger wraps transport errors rather
	// than replacing them, so the chain survives (see errors_test.go, which
	// asserts this against a real closed port).
	var (
		netErr net.Error
		urlErr *url.Error
		opErr  *net.OpError
		dnsErr *net.DNSError
	)
	return errors.As(err, &netErr) ||
		errors.As(err, &urlErr) ||
		errors.As(err, &opErr) ||
		errors.As(err, &dnsErr)
}

// DescribeConnectError turns a ValidateOrgAccess failure into an HTTP status and
// a message that names the actual cause, instead of asserting the token is bad.
//
// Callers should still log the underlying error — this returns only what is safe
// and useful to show a vendor admin.
func DescribeConnectError(err error, apiURL string) (int, string) {
	if IsUnreachable(err) {
		// Not an auth failure, so not 401: the upstream never answered.
		return http.StatusBadGateway, fmt.Sprintf(
			"Could not reach the Nuon API at %s. Check that the API URL is correct and that the API is running.",
			apiURL,
		)
	}

	// The API answered and refused. Surface what it said.
	apiErr := ParseAPIError(err)
	switch {
	case apiErr.Title != "" && apiErr.Description != "":
		return http.StatusUnauthorized, apiErr.Title + ": " + apiErr.Description
	case apiErr.Description != "":
		return http.StatusUnauthorized, apiErr.Description
	case apiErr.Title != "":
		return http.StatusUnauthorized, apiErr.Title
	default:
		return http.StatusUnauthorized, "Invalid API token or organization access"
	}
}
