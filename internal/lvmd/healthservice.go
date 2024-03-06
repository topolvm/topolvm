package lvmd

import (
	"context"

	"google.golang.org/grpc/health/grpc_health_v1"
)

func NewHealthService() grpc_health_v1.HealthServer {
	return &healthService{}
}

type healthService struct {
	grpc_health_v1.UnimplementedHealthServer
}

func (*healthService) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
}
