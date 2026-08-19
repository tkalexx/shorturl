package grpcserver

import (
	"context"
	"errors"

	"github.com/tkalexx/shorturl.git/internal/audit"
	"github.com/tkalexx/shorturl.git/internal/auth"
	"github.com/tkalexx/shorturl.git/internal/handler"
	pb "github.com/tkalexx/shorturl.git/pkg/api/shortener"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Server struct {
	pb.UnimplementedShortenerServiceServer
	service *handler.Service
	auditor *audit.Auditor
}

func New(service *handler.Service, auditor *audit.Auditor) *Server {
	if auditor == nil {
		auditor = audit.NewAuditor()
	}
	return &Server{service: service, auditor: auditor}
}

// ShortenURL реализует POST /api/shorten.
func (s *Server) ShortenURL(ctx context.Context, req *pb.URLShortenRequest) (*pb.URLShortenResponse, error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	shortURL, exists, err := s.service.Shorten(ctx, req.GetUrl(), userID)
	if err != nil {
		return nil, mapError(err)
	}

	s.auditor.LogShorten(userID, req.GetUrl())

	resp := &pb.URLShortenResponse{Result: shortURL}
	if exists {
		st := status.New(codes.AlreadyExists, "URL already exists")
		st, detailErr := st.WithDetails(resp)
		if detailErr != nil {
			return nil, status.Error(codes.AlreadyExists, shortURL)
		}
		return nil, st.Err()
	}
	return resp, nil
}

// ExpandURL реализует GET /{id}.
func (s *Server) ExpandURL(ctx context.Context, req *pb.URLExpandRequest) (*pb.URLExpandResponse, error) {
	if req == nil || req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	originalURL, err := s.service.Get(ctx, req.GetId())
	if err != nil {
		return nil, mapError(err)
	}

	userID := ""
	if id, ok := auth.UserIDFromContext(ctx); ok {
		userID = id
	}
	s.auditor.LogFollow(userID, originalURL)

	return &pb.URLExpandResponse{Result: originalURL}, nil
}

// ListUserURLs реализует GET /api/user/urls.
func (s *Server) ListUserURLs(ctx context.Context, _ *emptypb.Empty) (*pb.UserURLsResponse, error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	urls, err := s.service.GetUserURLs(ctx, userID)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal error")
	}

	resp := &pb.UserURLsResponse{
		Url: make([]*pb.URLData, 0, len(urls)),
	}
	for _, u := range urls {
		resp.Url = append(resp.Url, &pb.URLData{
			ShortUrl:    u.ShortURL,
			OriginalUrl: u.OriginalURL,
		})
	}
	return resp, nil
}

func mapError(err error) error {
	switch {
	case errors.Is(err, handler.ErrEmptyURL), errors.Is(err, handler.ErrInvalidURL):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, handler.ErrURLNotFound):
		return status.Error(codes.NotFound, "URL not found")
	case errors.Is(err, handler.ErrURLDeleted):
		return status.Error(codes.NotFound, "URL deleted")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}

// AuthInterceptor кладёт userID в контекст по metadata authorization.
// Для ShortenURL и ListUserURLs токен обязателен (при отсутствии выдаётся новый).
// Для ExpandURL токен опционален и используется только для аудита.
func AuthInterceptor(manager *auth.Manager) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		token := ""
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if values := md.Get(auth.AuthorizationMetadata); len(values) > 0 {
				token = values[0]
			}
		}

		requiresAuth := info.FullMethod == pb.ShortenerService_ShortenURL_FullMethodName ||
			info.FullMethod == pb.ShortenerService_ListUserURLs_FullMethodName

		if !requiresAuth {
			if token != "" {
				if userID, err := manager.UserIDFromToken(token); err == nil {
					ctx = auth.WithUserID(ctx, userID)
				}
			}
			return handler(ctx, req)
		}

		userID, issuedToken, tokenChanged, err := manager.Authenticate(token)
		if err != nil {
			if errors.Is(err, auth.ErrUnauthorized) {
				return nil, status.Error(codes.Unauthenticated, "unauthorized")
			}
			return nil, status.Error(codes.Internal, "internal error")
		}

		if tokenChanged {
			if err := grpc.SetHeader(ctx, metadata.Pairs(auth.AuthorizationMetadata, issuedToken)); err != nil {
				return nil, status.Error(codes.Internal, "internal error")
			}
		}

		ctx = auth.WithUserID(ctx, userID)
		return handler(ctx, req)
	}
}

// NewGRPCServer собирает grpc.Server с сервисом сокращения URL и auth-interceptor.
func NewGRPCServer(service *handler.Service, authManager *auth.Manager, auditor *audit.Auditor) *grpc.Server {
	srv := grpc.NewServer(grpc.UnaryInterceptor(AuthInterceptor(authManager)))
	pb.RegisterShortenerServiceServer(srv, New(service, auditor))
	return srv
}
