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

	"github.com/jointwt/twtxt"
	"github.com/jointwt/twtxt/types"
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
	if c.Config.Token != "" {
		req.Header.Set("Token", c.Config.Token)
	}
	return req, nil
}

func (c *Client) do(req *http.Request, v interface{}) error {
	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusInternalServerError:
		return ErrServerError
	}

	err = json.NewDecoder(res.Body).Decode(v)

	return err
}

// Login ...
func (c *Client) Login(username, password string) (res types.AuthResponse, err error) {
	req, err := c.newRequest("POST", "/auth", types.AuthRequest{username, password})
	if err != nil {
		return types.AuthResponse{}, err
	}
	err = c.do(req, &res)
	return
}

// Post ...
func (c *Client) Post(text string) (res types.AuthResponse, err error) {
	req, err := c.newRequest("POST", "/post", types.PostRequest{Text: text})
	if err != nil {
		return types.AuthResponse{}, err
	}
	err = c.do(req, &res)
	return
}

// Timeline ...
func (c *Client) Timeline(page int) (res types.PagedResponse, err error) {
	req, err := c.newRequest("POST", "/timeline", types.PagedRequest{Page: page})
	if err != nil {
		return types.PagedResponse{}, err
	}
	err = c.do(req, &res)
	return
}
