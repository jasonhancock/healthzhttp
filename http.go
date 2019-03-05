package healthzhttp

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/jasonhancock/healthz"
	"github.com/pkg/errors"
)

type options struct {
	client             *http.Client
	method             string
	matchContent       *regexp.Regexp
	body               []byte
	username           string
	password           string
	allowedStatusCodes map[int]struct{}
}

// Option is used to customize the summarizer
type Option func(*options) error

// WithHTTPClient sets the HTTP client to use for the checks
func WithHTTPClient(client *http.Client) Option {
	return func(o *options) error {
		o.client = client
		return nil
	}
}

// WithMethod sets the HTTP method to use for the checks
func WithMethod(method string) Option {
	return func(o *options) error {
		o.method = method
		return nil
	}
}

// WithBody sets the body to be sent along with each request
func WithBody(body []byte) Option {
	return func(o *options) error {
		o.body = body
		return nil
	}
}

// WithBasicAuth sets basic auth credentials to be used for each request
func WithBasicAuth(username, password string) Option {
	return func(o *options) error {
		o.username = username
		o.password = password
		return nil
	}
}

// WithAllowedStatusCode adds a status code that won't trigger an error
func WithAllowedStatusCode(status int) Option {
	return func(o *options) error {
		o.allowedStatusCodes[status] = struct{}{}
		return nil
	}
}

// WithoutAllowedStatusCode removes a status code from the list of allowed codes.
// This is useful to remove the default 200 status code
func WithoutAllowedStatusCode(status int) Option {
	return func(o *options) error {
		if _, ok := o.allowedStatusCodes[status]; ok {
			delete(o.allowedStatusCodes, status)
		}
		return nil
	}
}

// WithRegexp performs a content check on the body of the HTTP response.
func WithRegexp(expr string) Option {
	return func(o *options) error {
		regex, err := regexp.Compile(expr)
		if err != nil {
			return err
		}

		o.matchContent = regex
		return nil
	}
}

// CheckHTTP is an HTTP healthz check
type CheckHTTP struct {
	url                *url.URL
	client             *http.Client
	method             string
	body               []byte
	matchContent       *regexp.Regexp
	username           string
	password           string
	allowedStatusCodes map[int]struct{}
}

// NewCheck creates a new CheckHTTP.
func NewCheck(endpoint string, opts ...Option) (*CheckHTTP, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, errors.Wrap(err, "CheckHTTP parsing endpoint")
	}

	opt := &options{
		client: &http.Client{Timeout: 10 * time.Second},
		method: http.MethodGet,
		allowedStatusCodes: map[int]struct{}{
			http.StatusOK: struct{}{},
		},
	}
	for _, o := range opts {
		err := o(opt)
		if err != nil {
			return nil, errors.Wrap(err, "CheckHTTP evaluating options")
		}
	}

	c := &CheckHTTP{
		client:             opt.client,
		url:                u,
		method:             opt.method,
		body:               opt.body,
		allowedStatusCodes: opt.allowedStatusCodes,
		matchContent:       opt.matchContent,
	}

	return c, nil
}

// Check performs the check
func (c CheckHTTP) Check(ctx context.Context) *healthz.Response {
	body := bytes.NewReader(c.body)

	req, err := http.NewRequest(c.method, c.url.String(), body)
	if err != nil {
		return &healthz.Response{Error: err}
	}
	if c.username != "" && c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return &healthz.Response{Error: err}
	}
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return &healthz.Response{Error: errors.Wrap(err, "reading response body")}
	}

	if _, ok := c.allowedStatusCodes[resp.StatusCode]; !ok {
		return &healthz.Response{Error: errors.Errorf("Unexpected http status code: %d", resp.StatusCode)}
	}

	if c.matchContent != nil && !c.matchContent.Match(respBody) {
		return &healthz.Response{Error: errors.Errorf("the response body did not match the supplied regex: %s", c.matchContent.String())}
	}

	return &healthz.Response{}
}
