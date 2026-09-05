package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type dnsRCIClient struct {
	baseURL string
	http    *http.Client
}

type dnsRCIHTTPError struct {
	Method string
	Path   string
	Status int
	Body   string
}

func (e *dnsRCIHTTPError) Error() string {
	return fmt.Sprintf("RCI %s %s: http %d: %s", e.Method, e.Path, e.Status, strings.TrimSpace(e.Body))
}

type dnsRCIAppError struct {
	Path    string
	Message string
}

func (e *dnsRCIAppError) Error() string {
	return fmt.Sprintf("RCI %s: %s", e.Path, e.Message)
}

func newDNSRCIClient(baseURL string) *dnsRCIClient {
	return &dnsRCIClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *dnsRCIClient) getJSON(ctx context.Context, path string, out any) error {
	if !validRCIPath(path) {
		return fmt.Errorf("invalid RCI path %q", path)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("RCI GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("RCI GET %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		return &dnsRCIHTTPError{Method: http.MethodGet, Path: path, Status: resp.StatusCode, Body: string(body)}
	}
	if msg := dnsRCIErrorMessage(body); msg != "" {
		return &dnsRCIAppError{Path: path, Message: msg}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("RCI GET %s: decode: %w", path, err)
	}
	return nil
}

func (c *dnsRCIClient) deleteSetting(ctx context.Context, path string, params map[string]any) ([]byte, error) {
	if !validRCIPath(path) {
		return nil, fmt.Errorf("invalid RCI path %q", path)
	}
	query := url.Values{}
	for key, value := range params {
		key = strings.TrimSpace(key)
		if key == "" || strings.ContainsAny(key, "\r\n") {
			return nil, fmt.Errorf("invalid RCI query parameter %q", key)
		}
		switch v := value.(type) {
		case string:
			query.Set(key, v)
		case int:
			query.Set(key, fmt.Sprintf("%d", v))
		case int64:
			query.Set(key, fmt.Sprintf("%d", v))
		case float64:
			query.Set(key, fmt.Sprintf("%v", v))
		case bool:
			query.Set(key, fmt.Sprintf("%t", v))
		default:
			return nil, fmt.Errorf("unsupported RCI query value for %s", key)
		}
	}
	endpoint := c.baseURL + path
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("RCI DELETE %s: %w", path, err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("RCI DELETE %s: read: %w", path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseBody, &dnsRCIHTTPError{Method: http.MethodDelete, Path: path, Status: resp.StatusCode, Body: string(responseBody)}
	}
	if msg := dnsRCIErrorMessage(responseBody); msg != "" {
		return responseBody, &dnsRCIAppError{Path: path, Message: msg}
	}
	return responseBody, nil
}

func (c *dnsRCIClient) postJSON(ctx context.Context, path string, payload any) ([]byte, error) {
	if !validRCIPath(path) {
		return nil, fmt.Errorf("invalid RCI path %q", path)
	}
	command, err := dnsRCICommandPayload(path, payload)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(command)
	if err != nil {
		return nil, fmt.Errorf("RCI POST %s: marshal: %w", path, err)
	}
	// NDMS RCI mutations use one structured command tree POSTed to /rci/.
	// Keeping the command as JSON avoids shell/CLI interpolation and matches
	// Keenetic's own RCI transport semantics.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("RCI POST %s: %w", path, err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("RCI POST %s: read: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		return responseBody, &dnsRCIHTTPError{Method: http.MethodPost, Path: path, Status: resp.StatusCode, Body: string(responseBody)}
	}
	if msg := dnsRCIErrorMessage(responseBody); msg != "" {
		return responseBody, &dnsRCIAppError{Path: path, Message: msg}
	}
	return responseBody, nil
}

func dnsRCICommandPayload(path string, leaf any) (map[string]any, error) {
	if !validRCIPath(path) {
		return nil, fmt.Errorf("invalid RCI path %q", path)
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return nil, fmt.Errorf("empty RCI command path")
	}
	var node any = leaf
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if part == "" || strings.ContainsAny(part, "\r\n") {
			return nil, fmt.Errorf("invalid RCI command segment %q", part)
		}
		node = map[string]any{part: node}
	}
	command, ok := node.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("could not build RCI command payload")
	}
	return command, nil
}

func validRCIPath(path string) bool {
	return strings.HasPrefix(path, "/") && !strings.Contains(path, "..") && !strings.ContainsAny(path, "\r\n")
}

func dnsRCIErrorMessage(body []byte) string {
	if !bytes.Contains(bytes.ToLower(body), []byte(`"status"`)) {
		return ""
	}
	var root any
	if json.Unmarshal(body, &root) != nil {
		return ""
	}
	return dnsRCIErrorInValue(root)
}

func dnsRCIErrorInValue(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		if status, ok := typed["status"].(string); ok && strings.EqualFold(strings.TrimSpace(status), "error") {
			if message, ok := typed["message"].(string); ok && strings.TrimSpace(message) != "" {
				return strings.TrimSpace(message)
			}
			return "NDMS returned an application error"
		}
		for _, child := range typed {
			if msg := dnsRCIErrorInValue(child); msg != "" {
				return msg
			}
		}
	case []any:
		for _, child := range typed {
			if msg := dnsRCIErrorInValue(child); msg != "" {
				return msg
			}
		}
	}
	return ""
}
