package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/prologic/twtxt"
	"github.com/prologic/twtxt/internal"
)

var (
	// DefaultUserAgent ...
	DefaultUserAgent = fmt.Sprintf("twt/%s", twtxt.FullVersion())

	// ErrUnauthorized ...
	ErrUnauthorized = errors.New("error: authorization failed")

	// ErrServerError
	ErrServerError = errors.New("error: server error")
)

// Client ...
type Client struct {
	BaseURL   *url.URL
	Config    *Config
	UserAgent string

	httpClient *http.Client
}

// NewClient ...
func NewClient(options ...Option) (*Client, error) {
	config := NewConfig()

	for _, opt := range options {
		if err := opt(config); err != nil {
			return nil, err
		}
	}

	u, err := url.Parse(config.URI)
	if err != nil {
		return nil, err
	}

	cli := &Client{
		BaseURL:    u,
		Config:     config,
		UserAgent:  DefaultUserAgent,
		httpClient: http.DefaultClient,
	}

	return cli, nil
}

func (c *Client) newRequest(method, path string, body interface{}) (*http.Request, error) {
	path = strings.TrimPrefix(path, "/")
	rel := &url.URL{Path: path}
	u := c.BaseURL.ResolveReference(rel)
	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequest(method, u.String(), buf)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	return req, nil
}

func (c *Client) do(req *http.Request, v interface{}) (*http.Response, error) {
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case 401:
		return nil, ErrUnauthorized
	case 500:
		return nil, ErrServerError
	}

	err = json.NewDecoder(res.Body).Decode(v)

	return res, err
}

// Login ...
func (c *Client) Login(username, password string) (res internal.AuthResponse, err error) {
	req, err := c.newRequest("POST", "/auth", internal.AuthRequest{username, password})
	if err != nil {
		return internal.AuthResponse{}, err
	}
	_, err = c.do(req, &res)
	return
}
