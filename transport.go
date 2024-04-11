package transport

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/aws-sdk-go-v2/service/apigateway"
)

var (
	ErrResourceNotFound = errors.New("resource not found")
)

type Transport struct {
	apiID   string
	mapping resourceMapping

	client  *apigateway.Client
	log     *slog.Logger
	once    *sync.Once
	initErr error
}

func (t *Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	ctx := r.Context()

	if err := t.initMappings(); err != nil {
		return nil, err
	}

	t.log.DebugContext(ctx, "resources mapped", "resources", t.mapping)

	path := r.URL.Path
	// TODO: apigw stage

	resourceID, hasResource := t.mapping.matchResourceID(r.Method, path)
	if !hasResource {
		return nil, fmt.Errorf("%s for path %s", ErrResourceNotFound, r.URL.RequestURI())
	}

	input, err := createInvokeInput(r, t.apiID, resourceID, path)
	if err != nil {
		return nil, fmt.Errorf("create invoke input error: %w", err)
	}

	t.log.DebugContext(ctx, "invoke input created", invokeInputLogGroup(input))

	out, invokeErr := t.client.TestInvokeMethod(ctx, input)
	if invokeErr != nil {
		return nil, fmt.Errorf("invoke error: %w", invokeErr)
	}

	t.log.DebugContext(ctx, "invoke success", invokeOutputLogGroup(out))

	return createHTTPResponse(r, out), nil
}

func (t *Transport) initMappings() error {
	t.once.Do(func() {
		t.mapping, t.initErr = mapEndpointResources(t.client, t.apiID)
	})

	return t.initErr
}

func (t *Transport) Mappings() map[string]string {
	result := make(map[string]string, len(t.mapping))

	for k, r := range t.mapping {
		result[k] = fmt.Sprintf("%s->%s", r.id, r.regex.String())
	}

	return result
}

func NewTransport(client *apigateway.Client, apiID string, opts ...Option) *Transport {
	t := &Transport{
		apiID:  apiID,
		client: client,
		log:    nopLogger(),
		once:   new(sync.Once),
	}

	for _, opt := range opts {
		opt(t)
	}

	t.log = t.log.With(slog.String("rest_api_id", t.apiID))

	return t
}

func NewInitializedTransport(client *apigateway.Client, apiID string, opts ...Option) (*Transport, error) {
	t := NewTransport(client, apiID, opts...)

	if err := t.initMappings(); err != nil {
		return nil, fmt.Errorf("init mappings error: %w", err)
	}

	return t, nil
}

func createInvokeInput(r *http.Request, apiID, resourceID, path string) (*apigateway.TestInvokeMethodInput, error) {
	var body *string

	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, fmt.Errorf("read request body error: %w", err)
		}

		body = aws.String(string(bodyBytes))
	}

	input := &apigateway.TestInvokeMethodInput{
		HttpMethod:          aws.String(r.Method),
		ResourceId:          aws.String(resourceID),
		RestApiId:           aws.String(apiID),
		Body:                body,
		MultiValueHeaders:   r.Header,
		PathWithQueryString: aws.String(path + "?" + r.URL.Query().Encode()),
	}

	return input, nil
}

func createHTTPResponse(r *http.Request, out *apigateway.TestInvokeMethodOutput) *http.Response {
	return &http.Response{
		Status:        http.StatusText(int(out.Status)),
		StatusCode:    int(out.Status),
		Proto:         r.Proto,
		ProtoMajor:    r.ProtoMajor,
		ProtoMinor:    r.ProtoMinor,
		Header:        out.MultiValueHeaders,
		Body:          io.NopCloser(strings.NewReader(*out.Body)),
		ContentLength: int64(len(*out.Body)),
		Request:       r,
	}
}

type Option func(*Transport)

func WithLogger(l *slog.Logger) Option {
	return func(t *Transport) {
		t.log = l
	}
}
