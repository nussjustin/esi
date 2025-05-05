package esihttp_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"

	"github.com/nussjustin/esi/esihttp"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func newResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}

func errorTransport(err error) http.RoundTripper {
	return roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, err
	})
}

func fixedTransport(statusCode int, body string) http.RoundTripper {
	return roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return newResponse(statusCode, body), nil
	})
}

type brokenReader struct{}

var errBrokenReader = errors.New("broken reader")

func (brokenReader) Close() error {
	return nil
}

func (brokenReader) Read([]byte) (n int, err error) {
	return 0, errBrokenReader
}

func shortReadTransport() http.RoundTripper {
	return roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{Body: brokenReader{}}, nil
	})
}

func unreachableTransport() http.RoundTripper {
	return roundTripperFunc(func(*http.Request) (*http.Response, error) {
		panic("should not be called")
	})
}

func testClient(rt http.RoundTripper) *http.Client {
	return &http.Client{Transport: rt}
}

func createCookieJar(u *url.URL, cookies []*http.Cookie) http.CookieJar {
	jar, _ := cookiejar.New(nil)
	jar.SetCookies(u, cookies)
	return jar
}

type readOnlyCookieJar struct{ http.CookieJar }

func (r readOnlyCookieJar) SetCookies(*url.URL, []*http.Cookie) {
	panic("can not update cookies")
}

var testURL = &url.URL{
	Scheme: "https",
	Host:   "example.com",
	Path:   "/base",
}

func TestCookieJar(t *testing.T) {
	ctx := t.Context()

	if got := esihttp.CookieJar(ctx); got != nil {
		t.Errorf("got %v, want nil", got)
	}

	want := &readOnlyCookieJar{}

	ctx = esihttp.WithCookieJar(ctx, want)

	if got := esihttp.CookieJar(ctx); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestOriginalRequest(t *testing.T) {
	ctx := t.Context()

	if got := esihttp.OriginalRequest(ctx); got != nil {
		t.Errorf("got %v, want nil", got)
	}

	want := &http.Request{}

	ctx = esihttp.WithOriginalRequest(ctx, want)

	if got := esihttp.OriginalRequest(ctx); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestOriginalResponse(t *testing.T) {
	ctx := t.Context()

	if got := esihttp.OriginalResponse(ctx); got != nil { //nolint:bodyclose
		t.Errorf("got %v, want nil", got)
	}

	want := &http.Response{}

	ctx = esihttp.WithOriginalResponse(ctx, want)

	if got := esihttp.OriginalResponse(ctx); got != want { //nolint:bodyclose
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestClient(t *testing.T) {
	testCases := []struct {
		Name          string
		Client        esihttp.Client
		ContextFunc   func(context.Context) context.Context
		Expected      string
		ExpectedError error
	}{
		{
			Name: "2xx response",
			Client: esihttp.Client{
				HTTPClient: testClient(fixedTransport(http.StatusOK, "ok")),
			},
			Expected: "ok",
		},
		{
			Name: "4xx response",
			Client: esihttp.Client{
				HTTPClient: testClient(fixedTransport(http.StatusNotFound, "not found")),
			},
			ExpectedError: &esihttp.ClientError{StatusCode: http.StatusNotFound},
		},
		{
			Name: "4xx response with On4xx returning bytes",
			Client: esihttp.Client{
				HTTPClient: testClient(fixedTransport(http.StatusNotFound, "not found")),
				On4xx: func(*http.Response) ([]byte, error) {
					return []byte("fallback"), nil
				},
			},
			Expected: "fallback",
		},
		{
			Name: "4xx response with On4xx returning error",
			Client: esihttp.Client{
				HTTPClient: testClient(fixedTransport(http.StatusNotFound, "not found")),
				On4xx: func(*http.Response) ([]byte, error) {
					return nil, http.ErrNotSupported
				},
			},
			ExpectedError: http.ErrNotSupported,
		},
		{
			Name: "5xx response",
			Client: esihttp.Client{
				HTTPClient: testClient(fixedTransport(http.StatusBadGateway, "bad gateway")),
			},
			ExpectedError: &esihttp.ServerError{StatusCode: http.StatusBadGateway},
		},
		{
			Name: "5xx response with On5xx returning bytes",
			Client: esihttp.Client{
				HTTPClient: testClient(fixedTransport(http.StatusBadGateway, "bad gateway")),
				On5xx: func(*http.Response) ([]byte, error) {
					return []byte("fallback"), nil
				},
			},
			Expected: "fallback",
		},
		{
			Name: "5xx response with On5xx returning error",
			Client: esihttp.Client{
				HTTPClient: testClient(fixedTransport(http.StatusBadGateway, "bad gateway")),
				On5xx: func(*http.Response) ([]byte, error) {
					return nil, http.ErrNotSupported
				},
			},
			ExpectedError: http.ErrNotSupported,
		},
		{
			Name: "before request function",
			Client: esihttp.Client{
				HTTPClient: testClient(roundTripperFunc(func(r *http.Request) (*http.Response, error) {
					body := r.Header.Get("Extra-Header")

					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
				})),
				BeforeRequest: func(req *http.Request, _ map[string]string) error {
					req.Header.Set("Extra-Header", "extra data")
					return nil
				},
			},
			Expected: "extra data",
		},
		{
			Name: "before request function error",
			Client: esihttp.Client{
				HTTPClient: testClient(unreachableTransport()),
				BeforeRequest: func(*http.Request, map[string]string) error {
					return http.ErrSchemeMismatch
				},
			},
			ExpectedError: http.ErrSchemeMismatch,
		},
		{
			Name: "request error",
			Client: esihttp.Client{
				HTTPClient: testClient(errorTransport(http.ErrNotSupported)),
			},
			ExpectedError: http.ErrNotSupported,
		},
		{
			Name: "body read error",
			Client: esihttp.Client{
				HTTPClient: testClient(shortReadTransport()),
			},
			ExpectedError: errBrokenReader,
		},
		{
			Name: "url resolved from original request",
			Client: esihttp.Client{
				HTTPClient: testClient(fixedTransport(http.StatusOK, "ok")),
				BeforeRequest: func(req *http.Request, _ map[string]string) error {
					if got, want := req.URL.String(), "https://example.com/test"; got != want {
						panic(fmt.Sprintf("got URL %q, want %q", got, want))
					}

					return nil
				},
			},
			ContextFunc: func(ctx context.Context) context.Context {
				return esihttp.WithOriginalRequest(ctx, &http.Request{Method: "GET", URL: testURL})
			},
			Expected: "ok",
		},
		{
			Name: "cookies from jar",
			Client: esihttp.Client{
				HTTPClient: testClient(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					if c, err := req.Cookie("cookie1"); err != nil {
						panic(fmt.Sprintf("failed to read cookie1: %v", err))
					} else if c == nil {
						panic("missing cookie1")
					} else if c.Value != "value1" {
						panic(fmt.Sprintf("got cookie1 value %q, want %q", c.Value, "value1"))
					}

					if c, err := req.Cookie("cookie2"); err != nil {
						panic(fmt.Sprintf("failed to read cookie2: %v", err))
					} else if c == nil {
						panic("missing cookie2")
					} else if c.Value != "value2" {
						panic(fmt.Sprintf("got cookie1 value %q, want %q", c.Value, "value2"))
					}

					return newResponse(200, "ok"), nil
				})),
			},
			ContextFunc: func(ctx context.Context) context.Context {
				req := &http.Request{Method: "GET", URL: testURL}

				cookies := []*http.Cookie{
					{Name: "cookie1", Value: "value1"},
					{Name: "cookie2", Value: "value2"},
				}

				ctx = esihttp.WithCookieJar(ctx, readOnlyCookieJar{createCookieJar(req.URL, cookies)})
				ctx = esihttp.WithOriginalRequest(ctx, req)

				return ctx
			},
			Expected: "ok",
		},
		{
			Name: "cookies not updated",
			Client: esihttp.Client{
				HTTPClient: testClient(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					cookie := &http.Cookie{
						Name:   "test",
						Value:  "test",
						Path:   req.URL.Path,
						Domain: req.URL.Host,
					}

					resp := newResponse(200, "ok")
					resp.Header.Set("Set-Cookie", cookie.String())

					return resp, nil
				})),
			},
			ContextFunc: func(ctx context.Context) context.Context {
				req := &http.Request{Method: "GET", URL: testURL}

				var cookies []*http.Cookie

				ctx = esihttp.WithCookieJar(ctx, readOnlyCookieJar{createCookieJar(req.URL, cookies)})
				ctx = esihttp.WithOriginalRequest(ctx, req)

				return ctx
			},
			Expected: "ok",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			ctx := t.Context()

			if testCase.ContextFunc != nil {
				ctx = testCase.ContextFunc(ctx)
			}

			body, err := testCase.Client.Do(ctx, nil, "/test", map[string]string{"name": testCase.Name})

			if !errors.Is(err, testCase.ExpectedError) {
				t.Errorf("got error %v, want %v", err, testCase.ExpectedError)
			}

			if testCase.ExpectedError != nil {
				return
			}

			if got, want := string(body), testCase.Expected; got != want {
				t.Errorf("got body %v, want %v", got, want)
			}
		})
	}
}
