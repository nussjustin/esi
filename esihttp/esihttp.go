// Package esihttp implements aN HTTP based client for the esiproc package.
package esihttp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/nussjustin/esi/esiproc"
)

var cookieJarKey = new(int)

// CookieJar returns the cookie jar associated with the given context using [WithCookieJar].
func CookieJar(ctx context.Context) http.CookieJar {
	v, _ := ctx.Value(cookieJarKey).(http.CookieJar)
	return v
}

// WithCookieJar associates the given cookie jar with the context.
func WithCookieJar(ctx context.Context, jar http.CookieJar) context.Context {
	return context.WithValue(ctx, cookieJarKey, jar)
}

var origReqKey = new(int)

// OriginalRequest returns the request associated with the given context using [WithOriginalRequest].
func OriginalRequest(ctx context.Context) *http.Request {
	v, _ := ctx.Value(origReqKey).(*http.Request)
	return v
}

// WithOriginalRequest associates the given request with the context.
func WithOriginalRequest(ctx context.Context, origReq *http.Request) context.Context {
	return context.WithValue(ctx, origReqKey, origReq)
}

var origRespKey = new(int)

// OriginalResponse returns the response associated with the given context using [WithOriginalResponse].
func OriginalResponse(ctx context.Context) *http.Response {
	v, _ := ctx.Value(origRespKey).(*http.Response)
	return v
}

// WithOriginalResponse associates the given response with the context.
func WithOriginalResponse(ctx context.Context, origResp *http.Response) context.Context {
	return context.WithValue(ctx, origRespKey, origResp)
}

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
//
// See [Client.Do] for more information on how requests are configured.
type Client struct {
	// HTTPClient is used to make HTTP requests.
	//
	// The client should not have an associated cookie jar.
	//
	// If nil, [http.DefaultClient] is used.
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
//
// When the given context has an associated original request (see [WithOriginalRequest]), it is used to resolve the
// URL for the new request using [url.URL.ResolveReference].
//
// Similarly, if the context has an associated cookie jar (see [WithCookieJar]), it will be used to add cookies to the
// request. Note that cookies are only read from the jar, but not updated based on the response.
func (c *Client) Do(ctx context.Context, _ *esiproc.Processor, urlStr string, extra map[string]string) ([]byte, error) {
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	if baseReq := OriginalRequest(ctx); baseReq != nil {
		req.URL = baseReq.URL.ResolveReference(req.URL)
	}

	if cookieJar := CookieJar(ctx); cookieJar != nil {
		for _, c := range cookieJar.Cookies(req.URL) {
			req.AddCookie(c)
		}
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
