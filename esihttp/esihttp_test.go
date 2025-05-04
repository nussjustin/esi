package esihttp_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/nussjustin/esi/esihttp"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func errorTransport(err error) http.RoundTripper {
	return roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, err
	})
}

func fixedTransport(statusCode int, body string) http.RoundTripper {
	return roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: statusCode, Body: io.NopCloser(strings.NewReader(body))}, nil
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

func TestClient(t *testing.T) {
	testCases := []struct {
		Name          string
		Client        esihttp.Client
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
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			body, err := testCase.Client.Do(t.Context(), "/", map[string]string{"name": testCase.Name})

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
