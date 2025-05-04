// Package esihttp implements aN HTTP based client for the esiproc package.
package esihttp

import (
	"context"
	"errors"
	"fmt"
	"github.com/nussjustin/esi/esiproc"
	"io"
	"net/http"
)

// ClientError is returned by [Client.Do] when receiving a 4xx response and [Client.On4xx] is nil.
type ClientError struct {
	// StatusCode is the returned status code.
	StatusCode int
}

// Error returns a human-readable error message.
func (e *ClientError) Error() string {
	return fmt.Sprintf("unexpected status code: %d", e.StatusCode)
}

// Is returns true if the given error matches the receiver.
func (e *ClientError) Is(err error) bool {
	var o *ClientError
	return errors.As(err, &o) && *o == *e
}

// ServerError is returned by [Client.Do] when receiving a 5xx response and [Client.On5xx] is nil.
type ServerError struct {
	// StatusCode is the returned status code.
	StatusCode int
}

// Error returns a human-readable error message.
func (e *ServerError) Error() string {
	return fmt.Sprintf("unexpected status code: %d", e.StatusCode)
}

// Is returns true if the given error matches the receiver.
func (e *ServerError) Is(err error) bool {
	var o *ServerError
	return errors.As(err, &o) && *o == *e
}

// Client implements a [esiproc.Client] using HTTP to fetch data from URLs.
type Client struct {
	// HTTPClient is used to make HTTP requests.
	//
	// If nil, http.DefaultClient is used.
	HTTPClient *http.Client

	// BeforeRequest is called before sending the request and can be used to customize it.
	//
	// The extra map contains all extra attributes given to the <esi:include/> element.
	BeforeRequest func(req *http.Request, extra map[string]string) error

	// On4xx is called when receiving a request with a 4xx status code.
	//
	// Its return values are used as the return value for [Client.Do].
	//
	// If nil, a 4xx response will result in an [ClientError].
	On4xx func(resp *http.Response) ([]byte, error)

	// On5xx is called when receiving a request with a 5xx status code.
	//
	// Its return values are used as the return value for [Client.Do].
	//
	// If nil, a 5xx response will result in an [ServerError].
	On5xx func(resp *http.Response) ([]byte, error)
}

var _ esiproc.Client = (*Client)(nil)

// Do fetches data from the given URL and returns the response.
func (c *Client) Do(ctx context.Context, _ *esiproc.Processor, urlStr string, extra map[string]string) ([]byte, error) {
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	if c.BeforeRequest != nil {
		if err = c.BeforeRequest(req, extra); err != nil {
			return nil, err
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	switch {
	case resp.StatusCode >= 400 && resp.StatusCode <= 499 && c.On4xx != nil:
		return c.On4xx(resp)
	case resp.StatusCode >= 400 && resp.StatusCode <= 499:
		return nil, &ClientError{StatusCode: resp.StatusCode}
	case resp.StatusCode >= 500 && resp.StatusCode <= 599 && c.On5xx != nil:
		return c.On5xx(resp)
	case resp.StatusCode >= 500 && resp.StatusCode <= 599:
		return nil, &ServerError{StatusCode: resp.StatusCode}
	}

	return io.ReadAll(resp.Body)
}
