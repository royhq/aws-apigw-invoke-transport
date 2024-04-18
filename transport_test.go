package transport_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	transport "github.com/rcarrion2/aws-apigw-invoke-transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransport_RoundTrip(t *testing.T) {
	const apiID = "ugrux6gufp"

	t.Run("should map endpoint and return the response", func(t *testing.T) {
		testCases := map[string]struct {
			requestMethod        string
			requestURL           string
			requestBody          io.Reader
			expectedInvokeMethod string
			expectedInvokePath   string
			expectedInvokeBody   *string
			apiGwOut             *apigateway.TestInvokeMethodOutput
			expectedStatusCode   int
			expectedResponse     string
			expectedHeaders      http.Header
		}{
			"get returns 200 with custom domain": {
				requestMethod:        http.MethodGet,
				requestURL:           "https://custom-domain.com/api/v1/users/john.doe",
				requestBody:          http.NoBody,
				expectedInvokeMethod: http.MethodGet,
				expectedInvokePath:   "/api/v1/users/john.doe",
				apiGwOut: &apigateway.TestInvokeMethodOutput{
					Body:              aws.String(`{"username":"john.doe","age":23}`),
					MultiValueHeaders: map[string][]string{"Content-Type": {"application/json"}},
					Status:            http.StatusOK,
				},
				expectedStatusCode: http.StatusOK,
				expectedResponse:   `{"username":"john.doe","age":23}`,
				expectedHeaders:    map[string][]string{"Content-Type": {"application/json"}},
			},
			"get returns 200 with custom domain and query strings": {
				requestMethod:        http.MethodGet,
				requestURL:           "https://custom-domain.com/api/v1/users/john.doe?attrs=username&enabled=true",
				requestBody:          http.NoBody,
				expectedInvokeMethod: http.MethodGet,
				expectedInvokePath:   "/api/v1/users/john.doe?attrs=username&enabled=true",
				apiGwOut: &apigateway.TestInvokeMethodOutput{
					Body:              aws.String(`{"username":"john.doe","age":23}`),
					MultiValueHeaders: map[string][]string{"Content-Type": {"application/json"}},
					Status:            http.StatusOK,
				},
				expectedStatusCode: http.StatusOK,
				expectedResponse:   `{"username":"john.doe","age":23}`,
				expectedHeaders:    map[string][]string{"Content-Type": {"application/json"}},
			},
			"get returns 200 with invoke url": {
				requestMethod:        http.MethodGet,
				requestURL:           "https://ugrux6gufp.execute-api.us-east-1.amazonaws.com/stage/api/v1/users/john.doe",
				requestBody:          http.NoBody,
				expectedInvokeMethod: http.MethodGet,
				expectedInvokePath:   "/api/v1/users/john.doe",
				apiGwOut: &apigateway.TestInvokeMethodOutput{
					Body:              aws.String(`{"username":"john.doe","age":23}`),
					MultiValueHeaders: map[string][]string{"Content-Type": {"application/json"}},
					Status:            http.StatusOK,
				},
				expectedStatusCode: http.StatusOK,
				expectedResponse:   `{"username":"john.doe","age":23}`,
				expectedHeaders:    map[string][]string{"Content-Type": {"application/json"}},
			},
			"delete returns 204 with custom domain": {
				requestMethod:        http.MethodDelete,
				requestURL:           "https://custom-domain.com/api/v1/users/john.doe",
				requestBody:          http.NoBody,
				expectedInvokeMethod: http.MethodDelete,
				expectedInvokePath:   "/api/v1/users/john.doe",
				apiGwOut: &apigateway.TestInvokeMethodOutput{
					Body:              aws.String(""),
					MultiValueHeaders: map[string][]string{"Content-Type": {"application/json"}},
					Status:            http.StatusNoContent,
				},
				expectedStatusCode: http.StatusNoContent,
				expectedResponse:   "",
				expectedHeaders:    map[string][]string{"Content-Type": {"application/json"}},
			},
			"patch returns 204 with custom domain": {
				requestMethod:        http.MethodPatch,
				requestURL:           "https://custom-domain.com/api/v1/users",
				requestBody:          io.NopCloser(strings.NewReader(`{"enabled":false}`)),
				expectedInvokeMethod: http.MethodPatch,
				expectedInvokePath:   "/api/v1/users",
				expectedInvokeBody:   aws.String(`{"enabled":false}`),
				apiGwOut: &apigateway.TestInvokeMethodOutput{
					Body:              aws.String(""),
					MultiValueHeaders: map[string][]string{"Content-Type": {"application/json"}},
					Status:            http.StatusNoContent,
				},
				expectedStatusCode: http.StatusNoContent,
				expectedResponse:   "",
				expectedHeaders:    map[string][]string{"Content-Type": {"application/json"}},
			},
			"put returns 204 with custom domain": {
				requestMethod:        http.MethodPut,
				requestURL:           "https://custom-domain.com/api/v1/users",
				requestBody:          io.NopCloser(strings.NewReader(`{"username":"john.doe","age":23,"enabled":false}`)),
				expectedInvokeMethod: http.MethodPut,
				expectedInvokePath:   "/api/v1/users",
				expectedInvokeBody:   aws.String(`{"username":"john.doe","age":23,"enabled":false}`),
				apiGwOut: &apigateway.TestInvokeMethodOutput{
					Body:              aws.String(`{"username":"john.doe","age":23,"enabled":false}`),
					MultiValueHeaders: map[string][]string{"Content-Type": {"application/json"}},
					Status:            http.StatusNoContent,
				},
				expectedStatusCode: http.StatusNoContent,
				expectedResponse:   `{"username":"john.doe","age":23,"enabled":false}`,
				expectedHeaders:    map[string][]string{"Content-Type": {"application/json"}},
			},
		}

		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				apiGwCli := createApiGatewayClient(apiID)
				httpCli := &http.Client{Transport: transport.NewTransport(apiGwCli, apiID)}

				apiGwCli.setOutputIfMatch(func(i *apigateway.TestInvokeMethodInput) bool {
					match := *i.HttpMethod == tc.expectedInvokeMethod
					match = match && *i.PathWithQueryString == tc.expectedInvokePath

					if i.Body != nil {
						match = match && *i.Body == *tc.expectedInvokeBody
					} else {
						match = match && tc.expectedInvokeBody == nil
					}

					return match
				}, tc.apiGwOut)

				httpReq, err := http.NewRequest(tc.requestMethod, tc.requestURL, tc.requestBody)
				require.NoError(t, err, "failed to create request")

				httpResp, err := httpCli.Do(httpReq)
				require.NoError(t, err, "failed to make request")
				defer closeBody(httpResp)

				assert.Equal(t, tc.expectedStatusCode, httpResp.StatusCode, "status code mismatch")

				resp, _ := io.ReadAll(httpResp.Body)
				assert.Equal(t, tc.expectedResponse, string(resp), "response body mismatch")

				assert.Equal(t, len(tc.expectedResponse), int(httpResp.ContentLength),
					"response content length mismatch")
				assert.Len(t, httpResp.Header, len(tc.expectedHeaders),
					"response header length mismatch")

				for k := range httpResp.Header {
					if httpResp.Header.Get(k) != tc.expectedHeaders.Get(k) {
						assert.Equal(t, tc.expectedHeaders.Get(k), httpResp.Header.Get(k),
							"header mismatch")
					}
				}
			})
		}
	})

	t.Run("should return error when resource is not mapped", func(t *testing.T) {
		testCases := map[string]struct {
			requestMethod string
			requestURL    string
			requestBody   io.Reader
		}{
			"not mapped method": {
				requestMethod: http.MethodPost,
				requestURL:    "https://custom-domain.com/api/v1/users/john.doe",
				requestBody:   http.NoBody,
			},
			"not mapped resource": {
				requestMethod: http.MethodGet,
				requestURL:    "https://custom-domain.com/api/v1/posts",
				requestBody:   http.NoBody,
			},
		}

		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				apiGwCli := createApiGatewayClient(apiID)
				httpCli := &http.Client{Transport: transport.NewTransport(apiGwCli, apiID)}

				httpReq, err := http.NewRequest(tc.requestMethod, tc.requestURL, tc.requestBody)
				require.NoError(t, err, "failed to create request")

				httpResp, err := httpCli.Do(httpReq)
				defer closeBody(httpResp)

				assert.Zero(t, httpResp, "expected zero http response")
				assert.ErrorIs(t, err, transport.ErrResourceNotFound)
			})
		}
	})
}

func TestTransport_Mappings(t *testing.T) {
	const apiID = "ugrux6gufp"

	apiGwCli := createApiGatewayClient(apiID)
	httpTransport, err := transport.NewInitializedTransport(apiGwCli, apiID)
	require.NoError(t, err, "initialization failed")

	mappings := httpTransport.Mappings()

	expectedMappings := map[string]string{
		"GET#/api/v1/users/{value}":    "2cb3ff->^GET#/api/v1/users/([^/]+)$",
		"DELETE#/api/v1/users/{value}": "2cb3ff->^DELETE#/api/v1/users/([^/]+)$",
		"PATCH#/api/v1/users":          "8143a9->^PATCH#/api/v1/users$",
		"PUT#/api/v1/users":            "8143a9->^PUT#/api/v1/users$",
	}
	assert.Equal(t, expectedMappings, mappings)
}

func createApiGatewayClient(apiID string) *fakeApiGatewayClient {
	// /apu/v1/users/{value}	GET, DELETE
	// /api/v1/users			PUT, PATCH

	cli := &fakeApiGatewayClient{
		restApiID: apiID,
		region:    "us-east-1",
		resources: []types.Resource{
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
				ResourceMethods: map[string]types.Method{"PUT": {}, "PATCH": {}},
			},
			{
				Id:              aws.String("2cb3ff"),
				ParentId:        aws.String("8143a9"),
				Path:            aws.String("/api/v1/users/{value}"),
				PathPart:        aws.String("{value}"),
				ResourceMethods: map[string]types.Method{"GET": {}, "DELETE": {}},
			},
		},
	}

	return cli
}

type fakeApiGatewayClient struct {
	restApiID string
	region    string
	resources []types.Resource

	inputMatcher func(*apigateway.TestInvokeMethodInput) bool
	invokeOut    *apigateway.TestInvokeMethodOutput
	invokeErr    error
}

func (f *fakeApiGatewayClient) setOutputIfMatch(
	matcher func(*apigateway.TestInvokeMethodInput) bool,
	out *apigateway.TestInvokeMethodOutput,
) {
	f.inputMatcher = matcher
	f.invokeOut = out
	f.invokeErr = nil
}

func (f *fakeApiGatewayClient) setErrIfMatch(matcher func(*apigateway.TestInvokeMethodInput) bool, err error) {
	f.inputMatcher = matcher
	f.invokeOut = nil
	f.invokeErr = err
}

func (f *fakeApiGatewayClient) TestInvokeMethod(
	_ context.Context,
	input *apigateway.TestInvokeMethodInput,
	_ ...func(*apigateway.Options),
) (*apigateway.TestInvokeMethodOutput, error) {
	if *input.RestApiId != f.restApiID {
		return nil, errors.New("test: rest api not faked")
	}

	if f.inputMatcher == nil || f.inputMatcher(input) {
		return f.invokeOut, f.invokeErr
	}

	return nil, errors.New("test: could not match invoke input")
}

func (f *fakeApiGatewayClient) GetResources(
	_ context.Context,
	input *apigateway.GetResourcesInput,
	_ ...func(*apigateway.Options),
) (*apigateway.GetResourcesOutput, error) {
	if input != nil && *input.RestApiId == f.restApiID {
		return &apigateway.GetResourcesOutput{Items: f.resources}, nil
	}

	return nil, errors.New("test: rest api not faked")
}

func (f *fakeApiGatewayClient) Options() apigateway.Options {
	return apigateway.Options{Region: f.region}
}

func closeBody(r *http.Response) {
	if r != nil && r.Body != nil {
		_ = r.Body.Close()
	}
}
