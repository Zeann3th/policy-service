package grpc

import (
	"context"

	pb "github.com/zeann3th/policy-service/internal/application/grpc/codegen"
	cachecore "github.com/zeann3th/policy-service/internal/core/cache"
)

type cacheHandler struct {
	pb.UnimplementedCacheServiceServer
	service cachecore.Service
}

func (h *cacheHandler) ReloadCache(ctx context.Context, request *pb.ReloadCacheRequest) (*pb.ReloadCacheResponse, error) {
	response, err := h.service.Reload(ctx, cachecore.ReloadRequest{Key: request.Key, Namespaces: request.Namespaces, UpdatedBy: request.UpdatedBy})
	if err != nil {
		return nil, grpcError(err)
	}
	return &pb.ReloadCacheResponse{Evicted: response.Evicted, ReloadedNamespaces: response.ReloadedNamespaces}, nil
}
