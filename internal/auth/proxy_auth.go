// SPDX-License-Identifier: AGPL-3.0-only
package auth

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"net/textproto"
	"strings"
)

type AuthMode string

const (
	LocalAuth         AuthMode = "local"
	ProxyAuthOnly     AuthMode = "proxy"
	LocalAndProxyAuth AuthMode = "local_and_proxy"
)

type ProxyAuthConfig struct {
	Mode                           AuthMode
	IdentityHeader, AllowedSubject string
	ProxyCIDRs                     []string
}
type ProxyAuthenticator struct {
	config        ProxyAuthConfig
	identityPeers TrustedProxies
	cookieProxies TrustedProxies
}

func NewProxyAuthenticator(config ProxyAuthConfig, cookieProxies TrustedProxies) (*ProxyAuthenticator, error) {
	if config.Mode == "" {
		config.Mode = LocalAuth
	}
	if config.Mode != LocalAuth && config.Mode != ProxyAuthOnly && config.Mode != LocalAndProxyAuth {
		return nil, errors.New("auth mode must be local, proxy, or local_and_proxy")
	}
	peers, err := ParseTrustedProxies(config.ProxyCIDRs)
	if err != nil {
		return nil, err
	}
	if config.Mode != LocalAuth {
		config.IdentityHeader = textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(config.IdentityHeader))
		if config.IdentityHeader == "" || strings.ContainsAny(config.IdentityHeader, " \t\r\n:") {
			return nil, errors.New("proxy identity header is invalid")
		}
		if config.AllowedSubject == "" || len(config.AllowedSubject) > 512 || len(config.ProxyCIDRs) == 0 {
			return nil, errors.New("proxy authentication requires CIDRs, a header, and an allowed subject")
		}
	}
	return &ProxyAuthenticator{config: config, identityPeers: peers, cookieProxies: cookieProxies}, nil
}
func (p *ProxyAuthenticator) AllowsLocal() bool {
	return p == nil || p.config.Mode == LocalAuth || p.config.Mode == LocalAndProxyAuth
}
func (p *ProxyAuthenticator) AllowsProxy() bool {
	return p != nil && (p.config.Mode == ProxyAuthOnly || p.config.Mode == LocalAndProxyAuth)
}
func (p *ProxyAuthenticator) Mode() AuthMode {
	if p == nil {
		return LocalAuth
	}
	return p.config.Mode
}
func (p *ProxyAuthenticator) Subject(r *http.Request) (string, bool) {
	if !p.AllowsProxy() || !p.identityPeers.TrustedPeer(r) {
		return "", false
	}
	values := r.Header.Values(p.config.IdentityHeader)
	if len(values) != 1 || strings.Contains(values[0], ",") {
		return "", false
	}
	subject := values[0]
	if len(subject) != len(p.config.AllowedSubject) || subtle.ConstantTimeCompare([]byte(subject), []byte(p.config.AllowedSubject)) != 1 {
		return "", false
	}
	return subject, true
}
func (p *ProxyAuthenticator) SameOrigin(r *http.Request) bool {
	return p != nil && SameOrigin(r, p.cookieProxies)
}
func (p *ProxyAuthenticator) Secure(r *http.Request) bool {
	return p != nil && p.cookieProxies.Secure(r)
}
func (p *ProxyAuthenticator) CookieProxies() TrustedProxies {
	if p == nil {
		return TrustedProxies{}
	}
	return p.cookieProxies
}
