// Copyright 2019 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by the Polyform License
// that can be found in the LICENSE file.

package livelog

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v3"
)

const (
	endpointBatch  = "/stream?accountID=%s&key=%s"
	endpointUpload = "/blob?accountID=%s&key=%s"
)

var _ Client = (*HTTPClient)(nil)

var timeout = 10 * time.Second

// defaultClient is the default http.Client.
var defaultClient = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// New returns a new runner client.
func NewHTTPClient(endpoint, accountID, secret string, skipverify bool) *HTTPClient {
	client := &HTTPClient{
		Endpoint:   endpoint,
		AccountID:  accountID,
		Token:      secret,
		SkipVerify: skipverify,
	}
	if skipverify {
		client.Client = &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // nolint: gosec
				},
			},
		}
	}
	return client
}

// An HTTPClient manages communication with the runner API.
type HTTPClient struct {
	Client     *http.Client
	AccountID  string
	Endpoint   string
	Token      string
	SkipVerify bool
}

// Batch batch writes logs to the build logs.
func (c *HTTPClient) Batch(ctx context.Context, key string, lines []*Line) error { // nolint: stylecheck
	path := fmt.Sprintf(endpointBatch, c.AccountID, key)
	_, err := c.do(ctx, c.Endpoint+path, "PUT", &lines, nil) //nolint: bodyclose
	return err
}

func (c *HTTPClient) Upload(ctx context.Context, key string, r io.Reader) error {
	path := fmt.Sprintf(endpointUpload, c.AccountID, key)
	bckoff := createBackoff(timeout)
	_, err := c.retry(ctx, c.Endpoint+path, "POST", r, nil, true, bckoff) //nolint: bodyclose
	return err
}

// Open opens the data stream.
func (c *HTTPClient) Open(ctx context.Context, key string) error {
	path := fmt.Sprintf(endpointBatch, c.AccountID, key)
	bckoff := createBackoff(timeout)
	_, err := c.retry(ctx, c.Endpoint+path, "POST", nil, nil, false, bckoff) //nolint: bodyclose
	return err
}

// Close closes the data stream
func (c *HTTPClient) Close(ctx context.Context, key string) error {
	path := fmt.Sprintf(endpointBatch, c.AccountID, key)
	_, err := c.do(ctx, c.Endpoint+path, "DELETE", nil, nil) //nolint: bodyclose
	return err
}

func (c *HTTPClient) retry(ctx context.Context, method, path string, in, out interface{}, isOpen bool, b backoff.BackOff) (*http.Response, error) { // nolint: unparam
	for {
		var res *http.Response
		var err error
		if !isOpen {
			res, err = c.do(ctx, method, path, in, out)
		} else {
			res, err = c.open(ctx, method, path, in.(io.Reader))
		}

		// do not retry on Canceled or DeadlineExceeded
		if ctxErr := ctx.Err(); err != nil {
			return res, ctxErr
		}

		duration := b.NextBackOff()

		if res != nil {
			// Check the response code. We retry on 5xx-range
			// responses to allow the server time to recover, as
			// 5xx's are typically not permanent errors and may
			// relate to outages on the server side.

			if res.StatusCode >= http.StatusInternalServerError {
				if duration == backoff.Stop {
					return nil, err
				}
				time.Sleep(duration)
				continue
			}
		} else if err != nil {
			if duration == backoff.Stop {
				return nil, err
			}
			time.Sleep(duration)
			continue
		}
		return res, err
	}
}

// do is a helper function that posts a signed http request with
// the input encoded and response decoded from json.
func (c *HTTPClient) do(ctx context.Context, path, method string, in, out interface{}) (*http.Response, error) {
	var r io.Reader

	if in != nil {
		buf := new(bytes.Buffer)
		_ = json.NewEncoder(buf).Encode(in)
		r = buf
	}

	req, err := http.NewRequestWithContext(ctx, method, path, r)
	if err != nil {
		return nil, err
	}

	// the request should include the secret shared between
	// the agent and server for authorization.
	req.Header.Add("X-Harness-Token", c.Token)
	res, err := c.client().Do(req)
	if res != nil {
		defer func() {
			// drain the response body so we can reuse
			// this connection.
			_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096)) // nolint: gomnd
			res.Body.Close()
		}()
	}
	if err != nil {
		return res, err
	}

	// if the response body return no content we exit
	// immediately. We do not read or unmarshal the response
	// and we do not return an error.
	if res.StatusCode == http.StatusNoContent {
		return res, nil
	}

	// else read the response body into a byte slice.
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return res, err
	}

	if res.StatusCode >= http.StatusMultipleChoices {
		// if the response body includes an error message
		// we should return the error string.
		out := new(Error)
		if len(body) != 0 {
			if err := json.Unmarshal(body, out); err != nil {
				return res, new(Error)
			}
			return res, errors.New(
				string(body),
			)
		}
		// if the response body is empty we should return
		// the default status code text.
		return res, errors.New(
			http.StatusText(res.StatusCode),
		)
	}
	if out == nil {
		return res, nil
	}
	return res, json.Unmarshal(body, out)
}

// helper function to open an http request
func (c *HTTPClient) open(ctx context.Context, path, method string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("X-Harness-Token", c.Token)
	return c.client().Do(req)
}

// client is a helper function that returns the default client if a custom client is not defined.
func (p *HTTPClient) client() *http.Client { // nolint: revive
	if p.Client == nil {
		return defaultClient
	}
	return p.Client
}

func createBackoff(maxElapsedTime time.Duration) *backoff.ExponentialBackOff {
	exp := backoff.NewExponentialBackOff()
	exp.MaxElapsedTime = maxElapsedTime
	return exp
}