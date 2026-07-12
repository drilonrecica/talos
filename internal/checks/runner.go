// SPDX-License-Identifier: AGPL-3.0-only

package checks

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

type Resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type Runner struct {
	AllowPrivate bool
	Resolver     Resolver
	Dialer       *net.Dialer
	DialContext  func(context.Context, string, string) (net.Conn, error)
	Now          func() time.Time
}

var metadataAddresses = map[netip.Addr]struct{}{
	netip.MustParseAddr("169.254.169.254"): {}, netip.MustParseAddr("100.100.100.200"): {},
	netip.MustParseAddr("fd00:ec2::254"): {},
}

func (r *Runner) validateURL(ctx context.Context, raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return nil, fmt.Errorf("%s", FailureTargetBlocked)
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return nil, fmt.Errorf("%s", FailureTargetBlocked)
	}
	resolver := r.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	addrs, err := resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", FailureDNS, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("%s", FailureDNS)
	}
	for _, addr := range addrs {
		if !r.allowed(addr.Unmap()) {
			return nil, fmt.Errorf("%s", FailureTargetBlocked)
		}
	}
	return u, nil
}

func (r *Runner) allowed(addr netip.Addr) bool {
	if !addr.IsValid() || addr.IsUnspecified() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast() {
		return false
	}
	if _, blocked := metadataAddresses[addr]; blocked {
		return false
	}
	if addr.IsPrivate() && !r.AllowPrivate {
		return false
	}
	return true
}

func (r *Runner) Run(ctx context.Context, check Check) Result {
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	started := now()
	result := Result{CheckID: check.ID, Status: "failure", CheckedAt: started.UTC()}
	ctx, cancel := context.WithTimeout(ctx, check.Timeout)
	defer cancel()
	if _, err := r.validateURL(ctx, check.URL); err != nil {
		result.FailureCode = classify(err)
		return result
	}
	dialer := r.Dialer
	if dialer == nil {
		dialer = &net.Dialer{}
	}
	transport := &http.Transport{
		Proxy: nil, DisableKeepAlives: true, TLSHandshakeTimeout: check.Timeout,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			resolver := r.Resolver
			if resolver == nil {
				resolver = net.DefaultResolver
			}
			addrs, err := resolver.LookupNetIP(ctx, "ip", host)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", FailureDNS, err)
			}
			var last error
			for _, addr := range addrs {
				if !r.allowed(addr.Unmap()) {
					return nil, fmt.Errorf("%s", FailureTargetBlocked)
				}
				dial := r.DialContext
				if dial == nil {
					dial = dialer.DialContext
				}
				conn, dialErr := dial(ctx, network, net.JoinHostPort(addr.String(), port))
				if dialErr == nil {
					return conn, nil
				}
				last = dialErr
			}
			if last == nil {
				last = errors.New("no resolved addresses")
			}
			return nil, last
		},
	}
	client := &http.Client{Transport: transport, Timeout: check.Timeout}
	redirects := 0
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		redirects++
		if redirects > 3 {
			return errors.New("redirect limit exceeded")
		}
		_, err := r.validateURL(req.Context(), req.URL.String())
		return err
	}
	req, err := http.NewRequestWithContext(ctx, check.Method, check.URL, nil)
	if err != nil {
		result.FailureCode = FailureTargetBlocked
		return result
	}
	resp, err := client.Do(req)
	result.Latency = now().Sub(started)
	result.LatencyMS = result.Latency.Milliseconds()
	if err != nil {
		result.FailureCode = classify(err)
		return result
	}
	defer resp.Body.Close()
	result.HTTPStatus = resp.StatusCode
	if resp.StatusCode < check.ExpectedStatusMin || resp.StatusCode > check.ExpectedStatusMax {
		result.FailureCode = FailureUnexpectedStatus
		return result
	}
	if check.BodySubstring != "" && check.Method != http.MethodHead {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, MaxBodyRead+1))
		if readErr != nil {
			result.FailureCode = FailureConnection
			return result
		}
		if len(body) > MaxBodyRead || !bytes.Contains(body, []byte(check.BodySubstring)) {
			result.FailureCode = FailureBodyMismatch
			return result
		}
	}
	result.Status = "success"
	return result
}

func classify(err error) FailureCode {
	s := err.Error()
	for _, code := range []FailureCode{FailureTargetBlocked, FailureDNS} {
		if strings.Contains(s, string(code)) {
			return code
		}
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, syscall.ETIMEDOUT) {
		return FailureTimeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return FailureTimeout
	}
	var tlsErr tls.RecordHeaderError
	if errors.As(err, &tlsErr) || strings.Contains(strings.ToLower(s), "tls") || strings.Contains(strings.ToLower(s), "certificate") {
		return FailureTLS
	}
	return FailureConnection
}
