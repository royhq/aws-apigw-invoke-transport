package transport

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigateway/types"
)

// mapEndpointResources
func mapEndpointResources(cli ApiGwClient, apiID string) (resourceMapping, error) {
	ctx := context.Background()

	resources, err := cli.GetResources(ctx, &apigateway.GetResourcesInput{
		RestApiId: aws.String(apiID),
	})

	if err != nil {
		return nil, fmt.Errorf("get resources error: %w", err)
	}

	mapping := resourceMapping{}

	for _, res := range resources.Items {
		for method := range res.ResourceMethods {
			if err = mapping.add(res, method); err != nil {
				return nil, err
			}
		}
	}

	return mapping, nil
}

type resource struct {
	// id is the aws api gateway resource id.
	id    string
	regex *regexp.Regexp
}

type resourceMapping map[string]resource

func (mappings resourceMapping) matchResourceID(method, path string) (string, bool) {
	key := endpointKey(method, path)

	if r, found := mappings[key]; found {
		return r.id, true
	}

	for _, r := range mappings {
		if r.regex.MatchString(key) {
			return r.id, true
		}
	}

	return "", false
}

func (mappings resourceMapping) add(r types.Resource, method string) error {
	var (
		resourceID = *r.Id
		path       = *r.Path
		key        = endpointKey(method, path)
	)

	regex, err := resourceRegex(key)
	if err != nil {
		return err
	}

	mappings[key] = resource{id: resourceID, regex: regex}

	return nil
}

func (mappings resourceMapping) LogValue() slog.Value {
	attrs := make([]slog.Attr, 0, len(mappings))

	for k, r := range mappings {
		attrs = append(attrs, slog.Group(k,
			slog.String("resource_id", r.id),
			slog.String("pattern", r.regex.String())))
	}

	return slog.GroupValue(attrs...)
}

func resourceRegex(key string) (*regexp.Regexp, error) {
	pattern := regexp.QuoteMeta(key)
	pattern = regexp.MustCompile(`\\{[^/]+\}`).ReplaceAllString(pattern, `([^/]+)`)
	pattern = "^" + pattern + "$"

	regex, err := regexp.Compile(pattern)

	if err != nil {
		return nil, fmt.Errorf("could not compile resource regex: %w", err)
	}

	return regex, nil
}

func endpointKey(method, path string) string {
	return fmt.Sprintf("%s#%s", method, path) // e.g. POST#/path/to/resource
}
