package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	ext_authz_v3_core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	ext_authz_v3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	envoy_type "github.com/envoyproxy/go-control-plane/envoy/type/v3"

	"github.com/gogo/googleapis/google/rpc"
	"github.com/permitio/permit-golang/pkg/config"
	"github.com/permitio/permit-golang/pkg/enforcement"
	"github.com/permitio/permit-golang/pkg/permit"
	status "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"gopkg.in/square/go-jose.v2/jwt"
)

type authorizationServer struct {
	PermitClient *permit.Client
}

func newCheckResponse(code int32, httpStatus envoy_type.StatusCode, body string) *ext_authz_v3.CheckResponse {
	return &ext_authz_v3.CheckResponse{
		Status: &status.Status{
			Code: code,
		},
		HttpResponse: &ext_authz_v3.CheckResponse_DeniedResponse{
			DeniedResponse: &ext_authz_v3.DeniedHttpResponse{
				Status: &envoy_type.HttpStatus{Code: httpStatus},
				Body:   body,
			},
		},
	}
}

func parseAuthHeader(authHeader string) (string, error) {
	splitToken := strings.Split(authHeader, "Bearer ")
	if len(splitToken) != 2 {
		return "", fmt.Errorf("invalid authorization header format")
	}
	return splitToken[1], nil
}

func (a *authorizationServer) Check(ctx context.Context, req *ext_authz_v3.CheckRequest) (*ext_authz_v3.CheckResponse, error) {
	headers := req.GetAttributes().GetRequest().GetHttp().GetHeaders()
	authHeader := headers["authorization"]

	if authHeader == "" {
		return newCheckResponse(int32(rpc.UNAUTHENTICATED), 401, "Invalid authorization header"), nil
	}

	tokenStr, err := parseAuthHeader(authHeader)
	if err != nil {
		return newCheckResponse(int32(rpc.UNAUTHENTICATED), 401, "Unable to parse JWT"), nil
	}

	token, err := jwt.ParseSigned(tokenStr)
	if err != nil {
		return newCheckResponse(int32(rpc.UNAUTHENTICATED), 401, "Unable to parse JWT"), nil
	}

	var claims jwt.Claims
	if err := token.UnsafeClaimsWithoutVerification(&claims); err != nil {
		return newCheckResponse(int32(rpc.UNAUTHENTICATED), 401, "Unable to extract claims"), nil
	}

	resourceHeader := headers["x-app-resource"]
	action := map[string]string{
		"GET":    "read",
		"POST":   "write",
		"PUT":    "update",
		"DELETE": "delete",
	}[req.GetAttributes().GetRequest().GetHttp().GetMethod()]

	appAttributes := map[string]interface{}{
		"app_roles": []string{"admin"},
	}

	user := enforcement.UserBuilder(claims.Subject).WithAttributes(appAttributes)
	resource := enforcement.ResourceBuilder(resourceHeader)

	permitted, err := a.PermitClient.Check(user.Build(), enforcement.Action(action), resource.Build())
	if err != nil {
		return newCheckResponse(int32(rpc.UNAUTHENTICATED), 401, "Permit error"), nil
	}

	if permitted {
		return &ext_authz_v3.CheckResponse{
			Status: &status.Status{
				Code: int32(rpc.OK),
			},
			HttpResponse: &ext_authz_v3.CheckResponse_OkResponse{
				OkResponse: &ext_authz_v3.OkHttpResponse{
					Headers: []*ext_authz_v3_core.HeaderValueOption{
						{
							Header: &ext_authz_v3_core.HeaderValue{
								Key:   "X-APP-IAM",
								Value: "FOO",
							},
						},
					},
				},
			},
		}, nil
	}

	return newCheckResponse(int32(rpc.UNAUTHENTICATED), 401, "Unauthorized"), nil
}

func main() {
	pdpKey := os.Getenv("PDP_KEY")
	if pdpKey == "" {
		panic("Missing PDP Key")
	}

	pdpUrl := os.Getenv("PDP_URL")
	if pdpUrl == "" {
		panic("Missing PDP URL")
	}

	permitClient := permit.NewPermit(
		config.NewConfigBuilder(pdpKey).
			WithPdpUrl(pdpUrl).
			Build(),
	)

	fmt.Println("Starting server")
	lis, _ := net.Listen("tcp", ":9192")
	grpcServer := grpc.NewServer()
	authServer := &authorizationServer{PermitClient: permitClient}
	ext_authz_v3.RegisterAuthorizationServer(grpcServer, authServer)
	grpcServer.Serve(lis)
}
