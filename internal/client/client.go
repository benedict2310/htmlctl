package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/benedict2310/htmlctl/internal/transport"
)

const defaultBaseURL = "http://htmlservd"

type APIClient struct {
	transport transport.Transport
	baseURL   string
	actor     string
}

func New(tr transport.Transport) *APIClient {
	actor := strings.TrimSpace(os.Getenv("USER"))
	if actor == "" {
		actor = "htmlctl"
	}
	return &APIClient{
		transport: tr,
		baseURL:   defaultBaseURL,
		actor:     actor,
	}
}

func (c *APIClient) ListWebsites(ctx context.Context) (WebsitesResponse, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/websites", nil)
	if err != nil {
		return WebsitesResponse{}, err
	}
	var out WebsitesResponse
	if err := c.do(req, &out); err != nil {
		return WebsitesResponse{}, err
	}
	return out, nil
}

func (c *APIClient) ListEnvironments(ctx context.Context, website string) (EnvironmentsResponse, error) {
	path := fmt.Sprintf("/api/v1/websites/%s/environments", url.PathEscape(strings.TrimSpace(website)))
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return EnvironmentsResponse{}, err
	}
	var out EnvironmentsResponse
	if err := c.do(req, &out); err != nil {
		return EnvironmentsResponse{}, err
	}
	return out, nil
}

func (c *APIClient) ListReleases(ctx context.Context, website, environment string) (ReleasesResponse, error) {
	const pageSize = 200

	var out ReleasesResponse
	offset := 0
	for {
		page, err := c.ListReleasesPage(ctx, website, environment, pageSize, offset)
		if err != nil {
			return ReleasesResponse{}, err
		}
		if offset == 0 {
			out.Website = page.Website
			out.Environment = page.Environment
			out.ActiveReleaseID = page.ActiveReleaseID
		}
		out.Releases = append(out.Releases, page.Releases...)
		if len(page.Releases) < pageSize {
			break
		}
		offset += len(page.Releases)
	}
	out.Offset = 0
	out.Limit = len(out.Releases)
	return out, nil
}

func (c *APIClient) ListReleasesPage(ctx context.Context, website, environment string, limit, offset int) (ReleasesResponse, error) {
	path := fmt.Sprintf(
		"/api/v1/websites/%s/environments/%s/releases",
		url.PathEscape(strings.TrimSpace(website)),
		url.PathEscape(strings.TrimSpace(environment)),
	)
	query := url.Values{}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		query.Set("offset", strconv.Itoa(offset))
	}
	if encoded := query.Encode(); encoded != "" {
		path = path + "?" + encoded
	}
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return ReleasesResponse{}, err
	}
	var out ReleasesResponse
	if err := c.do(req, &out); err != nil {
		return ReleasesResponse{}, err
	}
	return out, nil
}

func (c *APIClient) GetStatus(ctx context.Context, website, environment string) (StatusResponse, error) {
	path := fmt.Sprintf(
		"/api/v1/websites/%s/environments/%s/status",
		url.PathEscape(strings.TrimSpace(website)),
		url.PathEscape(strings.TrimSpace(environment)),
	)
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return StatusResponse{}, err
	}
	var out StatusResponse
	if err := c.do(req, &out); err != nil {
		return StatusResponse{}, err
	}
	return out, nil
}

func (c *APIClient) GetDesiredStateManifest(ctx context.Context, website, environment string) (DesiredStateManifestResponse, error) {
	path := fmt.Sprintf(
		"/api/v1/websites/%s/environments/%s/manifest",
		url.PathEscape(strings.TrimSpace(website)),
		url.PathEscape(strings.TrimSpace(environment)),
	)
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return DesiredStateManifestResponse{}, err
	}
	var out DesiredStateManifestResponse
	if err := c.do(req, &out); err != nil {
		return DesiredStateManifestResponse{}, err
	}
	return out, nil
}

func (c *APIClient) ApplyBundle(ctx context.Context, website, environment string, bundle io.Reader, dryRun bool) (ApplyUploadResponse, error) {
	path := fmt.Sprintf(
		"/api/v1/websites/%s/environments/%s/apply",
		url.PathEscape(strings.TrimSpace(website)),
		url.PathEscape(strings.TrimSpace(environment)),
	)
	if dryRun {
		path += "?dry_run=true"
	}
	req, err := c.newRequest(ctx, http.MethodPost, path, bundle)
	if err != nil {
		return ApplyUploadResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-tar")

	var out ApplyUploadResponse
	if err := c.do(req, &out); err != nil {
		return ApplyUploadResponse{}, err
	}
	return out, nil
}

func (c *APIClient) CreateRelease(ctx context.Context, website, environment string) (ReleaseCreateResponse, error) {
	path := fmt.Sprintf(
		"/api/v1/websites/%s/environments/%s/releases",
		url.PathEscape(strings.TrimSpace(website)),
		url.PathEscape(strings.TrimSpace(environment)),
	)
	req, err := c.newRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return ReleaseCreateResponse{}, err
	}
	var out ReleaseCreateResponse
	if err := c.do(req, &out); err != nil {
		return ReleaseCreateResponse{}, err
	}
	return out, nil
}

func (c *APIClient) Rollback(ctx context.Context, website, environment string) (RollbackResponse, error) {
	path := fmt.Sprintf(
		"/api/v1/websites/%s/environments/%s/rollback",
		url.PathEscape(strings.TrimSpace(website)),
		url.PathEscape(strings.TrimSpace(environment)),
	)
	req, err := c.newRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return RollbackResponse{}, err
	}
	var out RollbackResponse
	if err := c.do(req, &out); err != nil {
		return RollbackResponse{}, err
	}
	return out, nil
}

func (c *APIClient) Promote(ctx context.Context, website, fromEnv, toEnv string) (PromoteResponse, error) {
	path := fmt.Sprintf(
		"/api/v1/websites/%s/promote",
		url.PathEscape(strings.TrimSpace(website)),
	)
	body, err := json.Marshal(map[string]string{
		"from": strings.TrimSpace(fromEnv),
		"to":   strings.TrimSpace(toEnv),
	})
	if err != nil {
		return PromoteResponse{}, fmt.Errorf("marshal promote payload: %w", err)
	}
	req, err := c.newRequest(ctx, http.MethodPost, path, bytes.NewReader(body))
	if err != nil {
		return PromoteResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	var out PromoteResponse
	if err := c.do(req, &out); err != nil {
		return PromoteResponse{}, err
	}
	return out, nil
}

func (c *APIClient) GetLogs(ctx context.Context, website, environment string, limit int) (LogsResponse, error) {
	path := fmt.Sprintf(
		"/api/v1/websites/%s/environments/%s/logs",
		url.PathEscape(strings.TrimSpace(website)),
		url.PathEscape(strings.TrimSpace(environment)),
	)
	if limit > 0 {
		path += "?limit=" + strconv.Itoa(limit)
	}
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return LogsResponse{}, err
	}
	var out LogsResponse
	if err := c.do(req, &out); err != nil {
		return LogsResponse{}, err
	}
	return out, nil
}

func (c *APIClient) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("build request %s %s: %w", method, path, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Actor", c.actor)
	return req, nil
}

func (c *APIClient) do(req *http.Request, out any) error {
	resp, err := c.transport.Do(req.Context(), req)
	if err != nil {
		return mapTransportError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return mapAPIError(resp)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode api response: %w", err)
	}
	return nil
}

type apiErrorPayload struct {
	Error   string   `json:"error"`
	Details []string `json:"details"`
}

func mapAPIError(resp *http.Response) error {
	payload := apiErrorPayload{}
	body, _ := io.ReadAll(resp.Body)
	if len(body) > 0 {
		_ = json.Unmarshal(body, &payload)
	}
	msg := strings.TrimSpace(payload.Error)
	if msg == "" {
		msg = strings.TrimSpace(string(body))
	}
	if msg == "" {
		msg = "request failed"
	}
	if len(payload.Details) > 0 {
		msg = msg + ": " + strings.Join(payload.Details, "; ")
	}

	switch resp.StatusCode {
	case http.StatusBadRequest:
		return fmt.Errorf("invalid request: %s", msg)
	case http.StatusNotFound:
		return fmt.Errorf("resource not found: %s (check website/environment and --context)", msg)
	case http.StatusConflict:
		return fmt.Errorf("conflict: %s (retry after resolving concurrent changes)", msg)
	case http.StatusServiceUnavailable:
		return fmt.Errorf("server unavailable: %s", msg)
	default:
		if resp.StatusCode >= 500 {
			return fmt.Errorf("server error (%d): %s (check htmlservd logs)", resp.StatusCode, msg)
		}
		return fmt.Errorf("request failed (%d): %s", resp.StatusCode, msg)
	}
}

func mapTransportError(err error) error {
	switch {
	case errors.Is(err, transport.ErrSSHAuth):
		return fmt.Errorf("ssh authentication failed: %w", err)
	case errors.Is(err, transport.ErrSSHHostKey):
		return fmt.Errorf("ssh host key verification failed: %w", err)
	case errors.Is(err, transport.ErrSSHAgentUnavailable):
		return fmt.Errorf("ssh agent unavailable: %w", err)
	case errors.Is(err, transport.ErrSSHUnreachable):
		return fmt.Errorf("ssh host unreachable: %w", err)
	case errors.Is(err, transport.ErrSSHTunnel):
		return fmt.Errorf("ssh tunnel failed: %w", err)
	default:
		return err
	}
}
