package nuon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// closedPortURL returns a URL on a port that is guaranteed to have nothing
// listening, so dialling it fails immediately with connection refused.
func closedPortURL(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return "http://" + addr
}

// TestIsUnreachableOnRealTransportError is the load-bearing test: it asserts
// that the error go-swagger actually produces for an unreachable host is still
// classifiable through errors.As. If go-swagger ever stops wrapping and returns
// an opaque error instead, this fails and IsUnreachable needs another strategy.
func TestIsUnreachableOnRealTransportError(t *testing.T) {
	client, err := NewClientWithURL("nuon_pat_fake", "org_fake", closedPortURL(t))
	if err != nil {
		t.Fatalf("unable to build client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.ValidateOrgAccess(ctx)
	if err == nil {
		t.Fatal("expected an error dialling a closed port")
	}
	if !IsUnreachable(err) {
		t.Fatalf("connection refused must be classified as unreachable, got: %#v (%v)", err, err)
	}
}

func TestIsUnreachableClassification(t *testing.T) {
	unreachable := map[string]error{
		"nil-wrapped url error": fmt.Errorf("wrapped: %w", &net.OpError{Op: "dial", Err: errors.New("connection refused")}),
		"deadline exceeded":     fmt.Errorf("wrapped: %w", context.DeadlineExceeded),
		"cancelled":             fmt.Errorf("wrapped: %w", context.Canceled),
		"dns failure":           fmt.Errorf("wrapped: %w", &net.DNSError{Err: "no such host", Name: "nope.invalid"}),
	}
	for name, err := range unreachable {
		if !IsUnreachable(err) {
			t.Errorf("%s: expected unreachable", name)
		}
	}

	reachable := map[string]error{
		"nil":               nil,
		"api rejection":     errors.New("&{Description:token is expired Error:unauthorized UserError:true}"),
		"plain wrapped msg": fmt.Errorf("invalid API token or org access: %w", errors.New("403 forbidden")),
	}
	for name, err := range reachable {
		if IsUnreachable(err) {
			t.Errorf("%s: expected NOT unreachable", name)
		}
	}
}

// TestDescribeConnectErrorForUnreachableAPI reproduces the failure that made the
// Connect Org modal report "Invalid API token" when ctl-api simply was not
// running on NUON_API_URL.
func TestDescribeConnectErrorForUnreachableAPI(t *testing.T) {
	apiURL := closedPortURL(t)

	client, err := NewClientWithURL("nuon_pat_fake", "org_fake", apiURL)
	if err != nil {
		t.Fatalf("unable to build client: %v", err)
	}
	err = client.ValidateOrgAccess(context.Background())
	if err == nil {
		t.Fatal("expected an error dialling a closed port")
	}

	status, msg := DescribeConnectError(err, apiURL)

	if status != http.StatusBadGateway {
		t.Errorf("an unreachable upstream is not an auth failure: got %d, want %d", status, http.StatusBadGateway)
	}
	if !strings.Contains(msg, apiURL) {
		t.Errorf("message must name the URL that failed so it is self-diagnosing: %q", msg)
	}
	if strings.Contains(strings.ToLower(msg), "invalid api token") {
		t.Errorf("must not blame the token when the API was never reached: %q", msg)
	}
}

func TestDescribeConnectErrorForAPIRejection(t *testing.T) {
	// Shape produced by the go-swagger client when the API answers and refuses.
	err := fmt.Errorf("invalid API token or org access: %w",
		errors.New("&{Description:the token has expired Error:token is expired UserError:true}"))

	status, msg := DescribeConnectError(err, "https://api.nuon.co")

	if status != http.StatusUnauthorized {
		t.Errorf("a rejection should be 401, got %d", status)
	}
	if !strings.Contains(msg, "Token Is Expired") {
		t.Errorf("expected the API's own reason to be surfaced, got %q", msg)
	}
}

func TestDescribeConnectErrorFallsBackWhenUnparseable(t *testing.T) {
	// ParseAPIError scrapes go-swagger's String() output; if that format ever
	// changes we must still say something rather than return an empty message.
	status, msg := DescribeConnectError(errors.New(""), "https://api.nuon.co")

	if status != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", status)
	}
	if msg == "" {
		t.Error("message must never be empty")
	}
}
