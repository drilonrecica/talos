// SPDX-License-Identifier: AGPL-3.0-only
package checks

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"
)

type resolver map[string][]netip.Addr

func (r resolver) LookupNetIP(_ context.Context, _ string, host string) ([]netip.Addr, error) {
	return r[host], nil
}

func responseDial(response string, addresses *[]string) func(context.Context, string, string) (net.Conn, error) {
	return func(_ context.Context, _ string, address string) (net.Conn, error) {
		*addresses = append(*addresses, address)
		client, server := net.Pipe()
		go func() {
			defer server.Close()
			reader := bufio.NewReader(server)
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if line == "\r\n" {
					break
				}
			}
			_, _ = fmt.Fprint(server, response)
		}()
		return client, nil
	}
}

func TestRunDialsValidatedResolvedAddress(t *testing.T) {
	addresses := []string{}
	runner := Runner{Resolver: resolver{"public.test": {netip.MustParseAddr("93.184.216.34")}}, DialContext: responseDial("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok", &addresses)}
	c := Check{ID: "x", ResourceID: "r", Name: "x", URL: "http://public.test/health", Method: "GET", Interval: 30 * time.Second, Timeout: time.Second, ExpectedStatusMin: 200, ExpectedStatusMax: 299, BodySubstring: "ok"}
	result := runner.Run(context.Background(), c)
	if result.Status != "success" {
		t.Fatalf("result=%+v", result)
	}
	if len(addresses) != 1 || addresses[0] != "93.184.216.34:80" {
		t.Fatalf("dial addresses=%v", addresses)
	}
}

func TestRunBoundsBodyAndClassifiesMismatch(t *testing.T) {
	addresses := []string{}
	body := strings.Repeat("x", MaxBodyRead+1)
	runner := Runner{Resolver: resolver{"public.test": {netip.MustParseAddr("93.184.216.34")}}, DialContext: responseDial(fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Length: %d\r\n\r\n%s", len(body), body), &addresses)}
	c := Check{ID: "x", ResourceID: "r", Name: "x", URL: "http://public.test", Method: "GET", Interval: 30 * time.Second, Timeout: time.Second, ExpectedStatusMin: 200, ExpectedStatusMax: 299, BodySubstring: "needle"}
	if got := runner.Run(context.Background(), c).FailureCode; got != FailureBodyMismatch {
		t.Fatalf("failure=%q", got)
	}
}
func TestTargetProtection(t *testing.T) {
	tests := []struct {
		name, url string
		private   bool
		want      FailureCode
	}{{"credentials", "https://user:pass@example.test", false, FailureTargetBlocked}, {"localhost", "http://service.localhost", false, FailureTargetBlocked}, {"private", "http://private.test", false, FailureTargetBlocked}, {"metadata always", "http://metadata.test", true, FailureTargetBlocked}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := Runner{AllowPrivate: tt.private, Resolver: resolver{"example.test": {netip.MustParseAddr("93.184.216.34")}, "private.test": {netip.MustParseAddr("10.0.0.1")}, "metadata.test": {netip.MustParseAddr("169.254.169.254")}}}
			c := Check{ID: "x", ResourceID: "r", Name: "x", URL: tt.url, Method: "GET", Interval: 30 * time.Second, Timeout: time.Second, ExpectedStatusMin: 200, ExpectedStatusMax: 399}
			if got := runner.Run(context.Background(), c).FailureCode; got != tt.want {
				t.Fatalf("failure=%q want %q", got, tt.want)
			}
		})
	}
}
func TestPrivateTargetRequiresOptIn(t *testing.T) {
	runner := Runner{AllowPrivate: true, Resolver: resolver{"private.test": {netip.MustParseAddr("10.0.0.1")}}}
	if _, err := runner.validateURL(context.Background(), "http://private.test"); err != nil {
		t.Fatalf("private target rejected after opt-in: %v", err)
	}
}
