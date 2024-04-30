package transport_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/rcarrion2/aws-apigw-invoke-transport"
)

func TestTransport_RoundTrip(t *testing.T) {
	const (
		apiID        = "ortup5gufx"
		customDomain = "https://custom-domain.com"
		invokeURL    = "https://" + apiID + ".execute-api.us-east-1.amazonaws.com/stage"
	)

	t.Run("should map endpoint to resource and return response", func(t *testing.T) {
		testCases := map[string]struct {
			method              string
			pathWithQueryString string
			body                io.Reader
			invokeOutput        *apigateway.TestInvokeMethodOutput
			expectedResourceID  string
			expectedStatusCode  int
			expectedBody        string
			expectedHeaders     http.Header
		}{
			"GET 200 with path value": {
				method:              http.MethodGet,
				pathWithQueryString: "/api/v1/users/john.doe",
				body:                http.NoBody,
				invokeOutput: &apigateway.TestInvokeMethodOutput{
					Body:              aws.String(`{"username":"john.doe","age":33}`),
					MultiValueHeaders: map[string][]string{"Content-Type": {"application/json"}},
					Status:            http.StatusOK,
				},
				expectedResourceID: "2cb3ff",
				expectedStatusCode: http.StatusOK,
				expectedBody:       `{"username":"john.doe","age":33}`,
				expectedHeaders:    http.Header{"Content-Type": {"application/json"}},
			},
			"GET 200 with path value and query string": {
				method:              http.MethodGet,
				pathWithQueryString: "/api/v1/users/john.doe?attributes=age",
				body:                http.NoBody,
				invokeOutput: &apigateway.TestInvokeMethodOutput{
					Body:              aws.String(`{"age":33}`),
					MultiValueHeaders: map[string][]string{"Content-Type": {"application/json"}},
					Status:            http.StatusOK,
				},
				expectedResourceID: "2cb3ff",
				expectedStatusCode: http.StatusOK,
				expectedBody:       `{"age":33}`,
				expectedHeaders:    http.Header{"Content-Type": {"application/json"}},
			},
			"DELETE 204 with path value": {
				method:              http.MethodDelete,
				pathWithQueryString: "/api/v1/users/john.doe?attributes=age",
				body:                http.NoBody,
				invokeOutput: &apigateway.TestInvokeMethodOutput{
					Body:              aws.String(""),
					MultiValueHeaders: map[string][]string{"Content-Type": {"application/json"}},
					Status:            http.StatusNoContent,
				},
				expectedResourceID: "2cb3ff",
				expectedStatusCode: http.StatusNoContent,
				expectedBody:       "",
				expectedHeaders:    http.Header{"Content-Type": {"application/json"}},
			},
			"POST 201": {
				method:              http.MethodPost,
				pathWithQueryString: "/api/v1/users",
				body:                strings.NewReader(`{"username":"john.doe",age":33}`),
				invokeOutput: &apigateway.TestInvokeMethodOutput{
					Body:              aws.String(`{"username":"john.doe","age":33}`),
					MultiValueHeaders: map[string][]string{"Content-Type": {"application/json"}},
					Status:            http.StatusCreated,
				},
				expectedResourceID: "8143a9",
				expectedStatusCode: http.StatusCreated,
				expectedBody:       `{"username":"john.doe","age":33}`,
				expectedHeaders:    http.Header{"Content-Type": {"application/json"}},
			},
			"PATCH 204": {
				method:              http.MethodPatch,
				pathWithQueryString: "/api/v1/users",
				body:                strings.NewReader(`{"age":34}`),
				invokeOutput: &apigateway.TestInvokeMethodOutput{
					Body:              aws.String(""),
					MultiValueHeaders: map[string][]string{"Content-Type": {"application/json"}},
					Status:            http.StatusNoContent,
				},
				expectedResourceID: "8143a9",
				expectedStatusCode: http.StatusNoContent,
				expectedBody:       "",
				expectedHeaders:    http.Header{"Content-Type": {"application/json"}},
			},
			"PUT 204": {
				method:              http.MethodPut,
				pathWithQueryString: "/api/v1/users",
				body:                strings.NewReader(`{"username":"john.doe","age":34}`),
				invokeOutput: &apigateway.TestInvokeMethodOutput{
					Body:              aws.String(""),
					MultiValueHeaders: map[string][]string{"Content-Type": {"application/json"}},
					Status:            http.StatusNoContent,
				},
				expectedResourceID: "8143a9",
				expectedStatusCode: http.StatusNoContent,
				expectedBody:       "",
				expectedHeaders:    http.Header{"Content-Type": {"application/json"}},
			},
		}

		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				test := func(t *testing.T, domain string) {
					// GIVEN
					httpReq := createRequest(tc.method, domain, tc.pathWithQueryString, tc.body)
					apiGwCli := new(apiGwClientMock)

					apiGwCli.
						On("GetResources", mock.MatchedBy(matchGetResourceInputForAPI(apiID))).
						Return(&apigateway.GetResourcesOutput{Items: createResources()}, nil).
						Once()

					apiGwCli.
						On("TestInvokeMethod", mock.MatchedBy(
							matchTestInvoke(apiID, tc.expectedResourceID, httpReq))).
						Return(tc.invokeOutput, nil).
						Once()

					tr := transport.NewTransport(apiGwCli, apiID)

					// WHEN
					httpResp, err := tr.RoundTrip(httpReq)

					// THEN
					require.NoError(t, err)

					assert.Equal(t, tc.expectedStatusCode, httpResp.StatusCode)

					respBody := readString(httpResp.Body)

					if tc.expectedBody != "" {
						assert.JSONEq(t, tc.expectedBody, respBody)
					} else {
						assert.Empty(t, respBody)
					}

					assert.Equal(t, tc.expectedHeaders, httpResp.Header)

					apiGwCli.AssertExpectations(t)
				}

				t.Run("with custom domain", func(t *testing.T) {
					test(t, customDomain)
				})

				t.Run("with invoke URL", func(t *testing.T) {
					test(t, invokeURL)
				})
			})
		}
	})

	t.Run("resource not found should return error", func(t *testing.T) {
		testCases := map[string]struct {
			method              string
			pathWithQueryString string
			body                io.Reader
		}{
			"non existent path": {
				method:              http.MethodGet,
				pathWithQueryString: "/api/v1/posts",
			},
			"existent path but method does not exist": {
				method:              http.MethodPost,
				pathWithQueryString: "/api/v1/users/john.doe",
			},
		}

		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				// GIVEN
				httpReq := createRequest(tc.method, customDomain, tc.pathWithQueryString, tc.body)
				apiGwCli := new(apiGwClientMock)

				apiGwCli.
					On("GetResources", mock.Anything).
					Return(&apigateway.GetResourcesOutput{Items: createResources()}, nil).
					Once()

				tr := transport.NewTransport(apiGwCli, apiID)

				// WHEN
				httpResp, err := tr.RoundTrip(httpReq)

				// THEN
				assert.Zero(t, httpResp)
				assert.ErrorIs(t, err, transport.ErrResourceNotFound)

				apiGwCli.AssertExpectations(t)
			})
		}
	})

	t.Run("invoke error should return error", func(t *testing.T) {
		// GIVEN
		httpReq := createRequest(http.MethodGet, customDomain, "/api/v1/users/john.doe", http.NoBody)
		apiGwCli := new(apiGwClientMock)

		invokeErr := errors.New("something went wrong")

		apiGwCli.
			On("GetResources", mock.MatchedBy(matchGetResourceInputForAPI(apiID))).
			Return(&apigateway.GetResourcesOutput{Items: createResources()}, nil).
			Once()

		apiGwCli.
			On("TestInvokeMethod", mock.Anything).
			Return(nil, invokeErr).
			Once()

		tr := transport.NewTransport(apiGwCli, apiID)

		// WHEN
		httpResp, err := tr.RoundTrip(httpReq)

		// THEN
		assert.Zero(t, httpResp)
		assert.ErrorIs(t, err, invokeErr)
		assert.EqualError(t, err, "invoke error: something went wrong")

		apiGwCli.AssertExpectations(t)
	})

	t.Run("get resources error should return error", func(t *testing.T) {
		t.Run("when create transport", func(t *testing.T) {
			// GIVEN
			apiGwCli := new(apiGwClientMock)

			getResourcesErr := errors.New("something went wrong")

			apiGwCli.
				On("GetResources", mock.MatchedBy(matchGetResourceInputForAPI(apiID))).
				Return(nil, getResourcesErr).
				Once()

			// WHEN
			tr, err := transport.NewInitializedTransport(apiGwCli, apiID)

			// THEN
			assert.Zero(t, tr)
			assert.ErrorIs(t, err, getResourcesErr)
			assert.EqualError(t, err, "get resources error: something went wrong")

			apiGwCli.AssertExpectations(t)
		})

		t.Run("when round trip", func(t *testing.T) {
			// GIVEN
			apiGwCli := new(apiGwClientMock)

			getResourcesErr := errors.New("something went wrong")

			apiGwCli.
				On("GetResources", mock.MatchedBy(matchGetResourceInputForAPI(apiID))).
				Return(nil, getResourcesErr).
				Once()

			tr := transport.NewTransport(apiGwCli, apiID)

			// WHEN
			httpResp, err := tr.RoundTrip(httptest.NewRequest(http.MethodGet, "/", http.NoBody))

			// THEN
			assert.Zero(t, httpResp)
			assert.ErrorIs(t, err, getResourcesErr)
			assert.EqualError(t, err, "get resources error: something went wrong")

			apiGwCli.AssertExpectations(t)
		})
	})

	t.Run("resource mapping should be executed once", func(t *testing.T) {
		testCases := map[string]func(transport.ApiGwClient) (*transport.Transport, error){
			"when mapping in transport creation": func(c transport.ApiGwClient) (*transport.Transport, error) {
				return transport.NewInitializedTransport(c, apiID)
			},
			"when map in round trip": func(c transport.ApiGwClient) (*transport.Transport, error) {
				return transport.NewTransport(c, apiID), nil
			},
		}

		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				// GIVEN
				httpReq := createRequest(http.MethodGet, customDomain, "/api/v1/users/john.doe", http.NoBody)
				apiGwCli := new(apiGwClientMock)

				apiGwCli.
					On("GetResources", mock.MatchedBy(matchGetResourceInputForAPI(apiID))).
					Return(&apigateway.GetResourcesOutput{Items: createResources()}, nil).
					Once()

				apiGwCli.
					On("TestInvokeMethod", mock.MatchedBy(
						matchTestInvoke(apiID, "2cb3ff", httpReq))).
					Return(&apigateway.TestInvokeMethodOutput{
						Body:   aws.String(""),
						Status: http.StatusOK,
					}, nil).
					Times(3)

				tr, err := tc(apiGwCli)
				require.NoError(t, err)

				// WHEN
				for range 3 {
					httpResp, err := tr.RoundTrip(httpReq)
					require.NoError(t, err)
					assert.Equal(t, http.StatusOK, httpResp.StatusCode)
				}

				// THEN
				apiGwCli.AssertExpectations(t)
			})
		}
	})
}

func TestTransport_Mappings(t *testing.T) {
	// GIVEN
	const apiID = "ortup5gufx"

	apiGwCli := new(apiGwClientMock)

	apiGwCli.
		On("GetResources", mock.MatchedBy(matchGetResourceInputForAPI(apiID))).
		Return(&apigateway.GetResourcesOutput{Items: createResources()}, nil).
		Once()

	httpTransport, err := transport.NewInitializedTransport(apiGwCli, apiID)
	require.NoError(t, err, "initialization failed")

	// WHEN
	mappings := httpTransport.Mappings()

	// THEN
	expectedMappings := map[string]string{
		"GET#/api/v1/users/{value}":    "2cb3ff->^GET#/api/v1/users/([^/]+)$",
		"DELETE#/api/v1/users/{value}": "2cb3ff->^DELETE#/api/v1/users/([^/]+)$",
		"POST#/api/v1/users":           "8143a9->^POST#/api/v1/users$",
		"PATCH#/api/v1/users":          "8143a9->^PATCH#/api/v1/users$",
		"PUT#/api/v1/users":            "8143a9->^PUT#/api/v1/users$",
	}

	assert.Equal(t, expectedMappings, mappings)

	apiGwCli.AssertExpectations(t)
}

func TestWithLogger(t *testing.T) {
	buf := new(bytes.Buffer)
	log := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	apiGwCli := new(apiGwClientMock)

	apiGwCli.
		On("GetResources", mock.MatchedBy(matchGetResourceInputForAPI("abc123"))).
		Return(&apigateway.GetResourcesOutput{Items: createResources()}, nil).
		Once()

	_, err := transport.NewInitializedTransport(apiGwCli, "abc123", transport.WithLogger(log))
	require.NoError(t, err)

	assert.Contains(t, buf.String(), `level=DEBUG msg="initializing endpoint mappings" rest_api_id=abc123`)
	assert.Contains(t, buf.String(), `level=DEBUG msg="mappings ready" rest_api_id=abc123`)
}

type apiGwClientMock struct{ mock.Mock }

func (m *apiGwClientMock) TestInvokeMethod(
	_ context.Context,
	input *apigateway.TestInvokeMethodInput,
	_ ...func(*apigateway.Options),
) (*apigateway.TestInvokeMethodOutput, error) {
	args := m.Called(input)

	var (
		out *apigateway.TestInvokeMethodOutput
		err error
	)

	if args.Get(0) != nil {
		out = args.Get(0).(*apigateway.TestInvokeMethodOutput)
	}

	if args.Get(1) != nil {
		err = args.Error(1)
	}

	return out, err
}

func (m *apiGwClientMock) GetResources(
	_ context.Context,
	input *apigateway.GetResourcesInput,
	_ ...func(*apigateway.Options),
) (*apigateway.GetResourcesOutput, error) {
	args := m.Called(input)

	var (
		out *apigateway.GetResourcesOutput
		err error
	)

	if args.Get(0) != nil {
		out = args.Get(0).(*apigateway.GetResourcesOutput)
	}

	if args.Get(1) != nil {
		err = args.Get(1).(error)
	}

	return out, err
}

func (m *apiGwClientMock) Options() apigateway.Options {
	return apigateway.Options{Region: "us-east-1"}
}

func createResources() []types.Resource {
	return []types.Resource{
		{
			Id:   aws.String("b7e20b3a4"),
			Path: aws.String("/"),
		},
		{
			Id:       aws.String("9b0826"),
			ParentId: aws.String("b7e20b3a4"),
			Path:     aws.String("/api"),
			PathPart: aws.String("api"),
		},
		{
			Id:       aws.String("7f4b77"),
			ParentId: aws.String("9b0826"),
			Path:     aws.String("/api/v1"),
			PathPart: aws.String("v1"),
		},
		{
			Id:              aws.String("8143a9"),
			ParentId:        aws.String("7f4b77"),
			Path:            aws.String("/api/v1/users"),
			PathPart:        aws.String("users"),
			ResourceMethods: map[string]types.Method{"POST": {}, "PUT": {}, "PATCH": {}},
		},
		{
			Id:              aws.String("2cb3ff"),
			ParentId:        aws.String("8143a9"),
			Path:            aws.String("/api/v1/users/{value}"),
			PathPart:        aws.String("{value}"),
			ResourceMethods: map[string]types.Method{"GET": {}, "DELETE": {}},
		},
	}
}

func createRequest(method, domain, path string, body io.Reader) *http.Request {
	httpReq, err := http.NewRequest(method, domain+path, body)
	if err != nil {
		panic(err)
	}

	httpReq.Header.Add("Content-Type", "application/json")
	httpReq.Header.Add("X-Request-ID", "0123456789")
	httpReq.Header.Add("X-User-Agent", "test_agent")

	return httpReq
}

func matchGetResourceInputForAPI(apiID string) func(*apigateway.GetResourcesInput) bool {
	return func(i *apigateway.GetResourcesInput) bool {
		return *i.RestApiId == apiID
	}
}

func matchTestInvoke(apiID, resourceID string, r *http.Request) func(*apigateway.TestInvokeMethodInput) bool {
	return func(i *apigateway.TestInvokeMethodInput) bool {
		if i == nil {
			return false
		}

		// match api && resource
		match := *i.RestApiId == apiID
		match = match && *i.ResourceId == resourceID

		// match method
		match = match && *i.HttpMethod == r.Method

		// match path
		pathWithQueryString := r.URL.Path + "?" + r.URL.RawQuery
		pathWithQueryString = strings.TrimSuffix(pathWithQueryString, "?")

		if r.Host == apiID+".execute-api.us-east-1.amazonaws.com" {
			pathWithQueryString = "/" + strings.Join(strings.Split(pathWithQueryString, "/")[2:], "/")
		}

		match = match && *i.PathWithQueryString == pathWithQueryString

		// match body
		if r.Body != nil && r.Body != http.NoBody {
			body := readString(r.Body)
			match = match && *i.Body == body
			r.Body = io.NopCloser(strings.NewReader(body))
		} else {
			match = match && i.Body == nil
		}

		// match headers
		match = match && len(i.MultiValueHeaders) == len(r.Header)
		for k, v := range i.MultiValueHeaders {
			match = match && v[0] == r.Header.Get(k)
		}

		return match
	}
}

func readString(r io.Reader) string {
	data, err := io.ReadAll(r)
	if err != nil {
		panic(err)
	}

	return string(data)
}
