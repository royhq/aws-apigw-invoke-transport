package transport

import (
	"io"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/service/apigateway"
)

func nopLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func invokeInputLogGroup(i *apigateway.TestInvokeMethodInput) slog.Attr {
	return slog.Group("api_gw_input",
		slog.String("resource_id", *i.ResourceId),
		slog.String("http_method", *i.HttpMethod),
		slog.String("path_with_query_string", *i.PathWithQueryString),
		slog.String("body", *i.Body),
		slog.Any("headers", i.Headers),
		slog.Any("multi_headers_value", i.MultiValueHeaders),
	)
}

func invokeOutputLogGroup(o *apigateway.TestInvokeMethodOutput) slog.Attr {
	return slog.Group("api_gw_output",
		slog.Int("status", int(o.Status)),
		slog.String("body", *o.Body),
		slog.Any("headers", o.Headers),
		slog.Any("multi_headers_value", o.MultiValueHeaders),
		slog.Int64("latency", o.Latency),
	)
}
