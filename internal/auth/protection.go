// SPDX-License-Identifier: AGPL-3.0-only
package auth

import (
	"container/list"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"
)

// TrustedProxies only affects forwarding-header provenance, never authentication.
type TrustedProxies struct{ prefixes []netip.Prefix }

func ParseTrustedProxies(values []string) (TrustedProxies, error) {
	p := TrustedProxies{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return TrustedProxies{}, err
		}
		if prefix.Bits() != prefix.Addr().BitLen() {
			return TrustedProxies{}, errors.New("trusted proxy CIDRs must identify one exact host (/32 for IPv4 or /128 for IPv6)")
		}
		p.prefixes = append(p.prefixes, prefix)
	}
	return p, nil
}
func (p TrustedProxies) trusted(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return p.trustedAddr(addr)
}

// TrustedPeer reports whether the immediate TCP peer is allowlisted. Forwarded
// headers are deliberately not considered.
func (p TrustedProxies) TrustedPeer(r *http.Request) bool { return p.trusted(r) }
func (p TrustedProxies) trustedAddr(addr netip.Addr) bool {
	for _, prefix := range p.prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
func (p TrustedProxies) Secure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if !p.trusted(r) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]), "https")
}
func (p TrustedProxies) ClientPrefix(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if p.trusted(r) {
		values := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
		if len(values) > 0 && len(values) <= 16 {
			for i := len(values) - 1; i >= 0; i-- {
				addr, parseErr := netip.ParseAddr(strings.TrimSpace(values[i]))
				if parseErr != nil {
					continue
				}
				if p.trustedAddr(addr) {
					continue
				}
				host = addr.String()
				break
			}
		}
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return "unknown"
	}
	if addr.Is4() {
		return netip.PrefixFrom(addr, 24).Masked().String()
	}
	return netip.PrefixFrom(addr, 56).Masked().String()
}

type BucketPolicy struct {
	Capacity float64
	Refill   time.Duration
}
type bucket struct {
	tokens float64
	at     time.Time
	used   time.Time
}
type bucketEntry struct {
	key   string
	value bucket
}
type Limiter struct {
	mu     sync.Mutex
	values map[string]*list.Element
	order  *list.List
	max    int
	now    func() time.Time
}

type Protection struct {
	limiter *Limiter
	proxies TrustedProxies
}

func NewProtection(max int, proxies TrustedProxies) *Protection {
	return &Protection{limiter: NewLimiter(max), proxies: proxies}
}
func (p *Protection) Proxies() TrustedProxies { return p.proxies }
func (p *Protection) allow(scope, key string, policy BucketPolicy) (bool, time.Duration) {
	return p.limiter.Allow(scope+":"+key, policy)
}
func (p *Protection) AllowLogin(r *http.Request, account string) (bool, time.Duration) {
	a, ra := p.allow("login-ip", p.proxies.ClientPrefix(r), BucketPolicy{Capacity: 10, Refill: time.Minute})
	b, rb := p.allow("login-account", strings.ToLower(strings.TrimSpace(account)), BucketPolicy{Capacity: 5, Refill: 5 * time.Minute})
	if rb > ra {
		ra = rb
	}
	return a && b, ra
}
func (p *Protection) AllowSetup(r *http.Request) (bool, time.Duration) {
	return p.allow("setup", p.proxies.ClientPrefix(r), BucketPolicy{Capacity: 5, Refill: 5 * time.Minute})
}
func (p *Protection) AllowDiagnostics(r *http.Request, actor string) (bool, time.Duration) {
	return p.allow("diagnostics", actor+":"+p.proxies.ClientPrefix(r), BucketPolicy{Capacity: 3, Refill: 15 * time.Minute})
}
func (p *Protection) AllowMetrics(r *http.Request) (bool, time.Duration) {
	return p.allow("metrics", p.proxies.ClientPrefix(r), BucketPolicy{Capacity: 60, Refill: time.Minute})
}
func (p *Protection) AllowEvents(r *http.Request) (bool, time.Duration) {
	return p.allow("events", p.proxies.ClientPrefix(r), BucketPolicy{Capacity: 60, Refill: time.Minute})
}
func (p *Protection) AllowResources(r *http.Request) (bool, time.Duration) {
	return p.allow("resources", p.proxies.ClientPrefix(r), BucketPolicy{Capacity: 120, Refill: time.Minute})
}
func (p *Protection) AllowLive(r *http.Request) (bool, time.Duration) {
	return p.allow("live", p.proxies.ClientPrefix(r), BucketPolicy{Capacity: 30, Refill: time.Minute})
}

func NewLimiter(max int) *Limiter {
	if max < 1 {
		max = 4096
	}
	return &Limiter{values: map[string]*list.Element{}, order: list.New(), max: max, now: time.Now}
}

// Allow is bounded even when an attacker presents unlimited distinct keys.
func (l *Limiter) Allow(key string, policy BucketPolicy) (bool, time.Duration) {
	if policy.Capacity <= 0 || policy.Refill <= 0 {
		return true, 0
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.values[key]
	if !ok {
		for l.order.Len() >= l.max {
			oldest := l.order.Back()
			delete(l.values, oldest.Value.(bucketEntry).key)
			l.order.Remove(oldest)
		}
		entry = l.order.PushFront(bucketEntry{key: key, value: bucket{tokens: policy.Capacity, at: now, used: now}})
		l.values[key] = entry
	}
	v := entry.Value.(bucketEntry)
	elapsed := now.Sub(v.value.at)
	v.value.tokens += elapsed.Seconds() * policy.Capacity / policy.Refill.Seconds()
	if v.value.tokens > policy.Capacity {
		v.value.tokens = policy.Capacity
	}
	v.value.at, v.value.used = now, now
	if v.value.tokens < 1 {
		entry.Value = v
		l.order.MoveToFront(entry)
		return false, time.Duration((1-v.value.tokens)*policy.Refill.Seconds()/policy.Capacity) * time.Second
	}
	v.value.tokens--
	entry.Value = v
	l.order.MoveToFront(entry)
	return true, 0
}

const CSRFCookieName = "binnacle_csrf"

func NewCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func CSRFHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawStdEncoding.EncodeToString(sum[:])
}
func SetCSRFCookie(w http.ResponseWriter, token string, secure bool, expires time.Time) {
	http.SetCookie(w, &http.Cookie{Name: CSRFCookieName, Value: token, Path: "/", HttpOnly: false, Secure: secure, SameSite: http.SameSiteLaxMode, Expires: expires.UTC()})
}
func ClearCSRFCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{Name: CSRFCookieName, Value: "", Path: "/", Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
}
func ValidCSRF(r *http.Request, expectedHash string) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
		return true
	}
	cookie, err := r.Cookie(CSRFCookieName)
	if err != nil {
		return false
	}
	header := r.Header.Get("X-CSRF-Token")
	if header == "" || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(CSRFHash(header)), []byte(expectedHash)) == 1
}

func SameOrigin(r *http.Request, proxies TrustedProxies) bool {
	raw := r.Header.Get("Origin")
	if raw == "" {
		raw = r.Referer()
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	scheme := "http"
	if proxies.Secure(r) {
		scheme = "https"
	}
	return strings.EqualFold(u.Scheme, scheme) && strings.EqualFold(u.Host, r.Host)
}
