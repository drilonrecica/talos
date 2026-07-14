// SPDX-License-Identifier: AGPL-3.0-only
package coolify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/drilonrecica/binnacle/internal/outbound"
)

const (
	MaxAPIResponseBytes    = 4 << 20
	MaxEnrichmentResources = 10000
)

type APIClient struct {
	base  *url.URL
	token string
	http  *http.Client
	sem   chan struct{}
}

type ClientConfig struct {
	BaseURL, Token    string
	AllowInsecureHTTP bool
	HTTPClient        *http.Client
}

func NewAPIClient(config ClientConfig) (*APIClient, error) {
	if strings.TrimSpace(config.Token) == "" {
		return nil, errors.New("Coolify API token is required")
	}
	base, err := url.Parse(strings.TrimRight(config.BaseURL, "/"))
	if err != nil || base.Hostname() == "" || base.User != nil {
		return nil, errors.New("invalid Coolify URL")
	}
	if base.Scheme != "https" && !(config.AllowInsecureHTTP && base.Scheme == "http") {
		return nil, errors.New("Coolify URL must use HTTPS")
	}
	if !strings.HasSuffix(base.Path, "/api/v1") {
		base.Path = strings.TrimRight(base.Path, "/") + "/api/v1"
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		policy := outbound.Policy{AllowPrivate: true}
		httpClient = &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{Proxy: nil, DialContext: policy.DialContext, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 10 * time.Second, DisableCompression: true}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	return &APIClient{base: base, token: config.Token, http: httpClient, sem: make(chan struct{}, 2)}, nil
}

type selectedResource struct {
	UUID          string `json:"uuid"`
	Name          string `json:"name"`
	FQDN          string `json:"fqdn"`
	Status        string `json:"status"`
	EnvironmentID int64  `json:"environment_id"`
	ServiceType   string `json:"service_type"`
	Project       *struct {
		UUID string `json:"uuid"`
		Name string `json:"name"`
	} `json:"project"`
	Environment *struct {
		Name      string `json:"name"`
		ProjectID int64  `json:"project_id"`
	} `json:"environment"`
}
type selectedProject struct {
	ID           int64 `json:"id"`
	UUID, Name   string
	Environments []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"environments"`
}
type selectedDeployment struct {
	UUID            string `json:"deployment_uuid"`
	ApplicationUUID string `json:"application_uuid"`
	Status          string `json:"status"`
	Commit          string `json:"commit"`
	CommitMessage   string `json:"commit_message"`
}

type ResourceMetadata struct {
	UUID        string   `json:"uuid"`
	Name        string   `json:"name"`
	Project     string   `json:"project,omitempty"`
	Environment string   `json:"environment,omitempty"`
	Domains     []string `json:"domains,omitempty"`
	Category    string   `json:"category"`
	Status      string   `json:"status,omitempty"`
}
type DeploymentMetadata struct{ UUID, ResourceUUID, Status, Commit, CommitMessage string }

func (c *APIClient) Metadata(ctx context.Context) ([]ResourceMetadata, error) {
	var projects []selectedProject
	var applications, services, databases []selectedResource
	tasks := []struct {
		path string
		dst  any
	}{{"/projects", &projects}, {"/applications", &applications}, {"/services", &services}, {"/databases", &databases}}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	errs := make(chan error, len(tasks))
	for _, task := range tasks {
		task := task
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.get(ctx, task.path, task.dst); err != nil {
				errs <- fmt.Errorf("%s: %w", task.path, err)
				cancel()
			}
		}()
	}
	wg.Wait()
	close(errs)
	if err := <-errs; err != nil {
		return nil, err
	}
	environments, projectNames := map[int64]string{}, map[int64]string{}
	for _, project := range projects {
		for _, environment := range project.Environments {
			environments[environment.ID] = environment.Name
			projectNames[environment.ID] = project.Name
		}
	}
	result := make([]ResourceMetadata, 0, len(applications)+len(services)+len(databases))
	appendResources := func(values []selectedResource, category string) error {
		for _, value := range values {
			if len(result) >= MaxEnrichmentResources {
				return errors.New("Coolify resource limit exceeded")
			}
			if value.UUID == "" {
				continue
			}
			project, environment := projectNames[value.EnvironmentID], environments[value.EnvironmentID]
			if value.Project != nil {
				project = value.Project.Name
			}
			if value.Environment != nil {
				environment = value.Environment.Name
			}
			actualCategory := category
			if category == "service" && strings.Contains(strings.ToLower(value.ServiceType), "database") {
				actualCategory = "database"
			}
			result = append(result, ResourceMetadata{UUID: value.UUID, Name: bounded(value.Name, 256), Project: bounded(project, 256), Environment: bounded(environment, 256), Domains: safeDomains(value.FQDN), Category: actualCategory, Status: bounded(value.Status, 64)})
		}
		return nil
	}
	if err := appendResources(applications, "application"); err != nil {
		return nil, err
	}
	if err := appendResources(services, "service"); err != nil {
		return nil, err
	}
	if err := appendResources(databases, "database"); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *APIClient) Deployments(ctx context.Context) ([]DeploymentMetadata, error) {
	var selected []selectedDeployment
	if err := c.get(ctx, "/deployments", &selected); err != nil {
		return nil, err
	}
	if len(selected) > MaxEnrichmentResources {
		return nil, errors.New("Coolify deployment limit exceeded")
	}
	result := make([]DeploymentMetadata, 0, len(selected))
	for _, value := range selected {
		if value.UUID != "" {
			result = append(result, DeploymentMetadata{bounded(value.UUID, 128), bounded(value.ApplicationUUID, 128), bounded(value.Status, 64), bounded(value.Commit, 128), bounded(value.CommitMessage, 512)})
		}
	}
	return result, nil
}

func (c *APIClient) get(ctx context.Context, path string, dst any) error {
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return ctx.Err()
	}
	target := *c.base
	target.Path = strings.TrimRight(c.base.Path, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Binnacle-Coolify/1")
	response, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return fmt.Errorf("Coolify returned HTTP %d", response.StatusCode)
	}
	reader := &limitReader{reader: response.Body, remaining: MaxAPIResponseBytes}
	if err = json.NewDecoder(reader).Decode(dst); err != nil {
		return err
	}
	if reader.exceeded {
		return errors.New("Coolify response exceeded limit")
	}
	return nil
}

type limitReader struct {
	reader    io.Reader
	remaining int64
	exceeded  bool
}

func (r *limitReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		r.exceeded = true
		return 0, errors.New("response too large")
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.reader.Read(p)
	r.remaining -= int64(n)
	return n, err
}
func bounded(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) > max {
		return value[:max]
	}
	return value
}
func safeDomains(raw string) []string {
	result := []string{}
	for _, value := range strings.Split(raw, ",") {
		u, err := url.Parse(strings.TrimSpace(value))
		if err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Hostname() != "" && len(result) < 16 {
			result = append(result, strings.ToLower(u.Hostname()))
		}
	}
	return result
}
