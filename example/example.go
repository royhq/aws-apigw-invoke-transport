package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/rcarrion2/aws-apigw-invoke-transport"
)

func main() {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithSharedConfigProfile("test-profile"), // profile from credentials file
		config.WithRegion("us-east-1"),
	)

	if err != nil {
		panic(err)
	}

	cli := apigateway.NewFromConfig(cfg)

	t := transport.NewTransport(cli, "uvx6fpruqg") // AWS API Gateway ID

	httpCli := &http.Client{
		Transport: t,
	}

	httpResp, err := httpCli.Get("https://any.com/api/v1/users/new_user_1642793267?key=UserName-index")
	if err != nil {
		panic(err)
	}

	dump, _ := httputil.DumpResponse(httpResp, true)
	fmt.Println("response:", string(dump))
	fmt.Println("mappings:", t.Mappings())
}
