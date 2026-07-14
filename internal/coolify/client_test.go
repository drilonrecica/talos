// SPDX-License-Identifier: AGPL-3.0-only
package coolify

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestClientSelectsSafeMetadataAndUsesReadBearerToken(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer team-read-token" {
			t.Errorf("authorization=%q", r.Header.Get("Authorization"))
			w.WriteHeader(401)
			return
		}
		mu.Lock()
		seen[r.URL.Path]++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/projects":
			fmt.Fprint(w, `[{"id":1,"uuid":"project-uuid","name":"Project","environments":[{"id":9,"name":"production"}]}]`)
		case "/api/v1/applications":
			fmt.Fprint(w, `[{"uuid":"app-uuid","name":"API","fqdn":"https://api.example.test","status":"running","environment_id":9,"docker_compose":"SECRET_COMPOSE","environment_variables":[{"value":"SECRET"}]}]`)
		case "/api/v1/services":
			fmt.Fprint(w, `[]`)
		case "/api/v1/databases":
			fmt.Fprint(w, `[]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewAPIClient(ClientConfig{BaseURL: server.URL, Token: "team-read-token", AllowInsecureHTTP: true, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	values, err := client.Metadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].UUID != "app-uuid" || values[0].Project != "Project" || values[0].Environment != "production" || values[0].Domains[0] != "api.example.test" {
		t.Fatalf("metadata=%+v", values)
	}
	payload := fmt.Sprintf("%+v", values)
	if strings.Contains(payload, "SECRET") {
		t.Fatalf("sensitive response field retained: %s", payload)
	}
	if len(seen) != 4 {
		t.Fatalf("endpoints=%v", seen)
	}
}

func TestClientRejectsRedirectAndOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/projects") {
			http.Redirect(w, r, "/secret", 302)
			return
		}
		fmt.Fprint(w, "[]")
	}))
	defer server.Close()
	httpClient := server.Client()
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	client, _ := NewAPIClient(ClientConfig{BaseURL: server.URL, Token: "token", AllowInsecureHTTP: true, HTTPClient: httpClient})
	if _, err := client.Metadata(context.Background()); err == nil {
		t.Fatal("redirect was accepted")
	}

	large := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "[\""+strings.Repeat("x", MaxAPIResponseBytes)+"\"]")
	}))
	defer large.Close()
	client, _ = NewAPIClient(ClientConfig{BaseURL: large.URL, Token: "token", AllowInsecureHTTP: true, HTTPClient: large.Client()})
	if err := client.get(context.Background(), "/anything", &[]string{}); err == nil {
		t.Fatal("oversized response was accepted")
	}
}

func TestClientRequiresHTTPSByDefault(t *testing.T) {
	if _, err := NewAPIClient(ClientConfig{BaseURL: "http://coolify.example.test", Token: "token"}); err == nil {
		t.Fatal("insecure HTTP accepted")
	}
}
