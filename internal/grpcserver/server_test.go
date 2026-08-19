package grpcserver_test

import (
	"context"
	"net"
	"testing"

	"github.com/tkalexx/shorturl.git/internal/auth"
	"github.com/tkalexx/shorturl.git/internal/grpcserver"
	"github.com/tkalexx/shorturl.git/internal/handler"
	"github.com/tkalexx/shorturl.git/internal/repository"
	pb "github.com/tkalexx/shorturl.git/pkg/api/shortener"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

const bufSize = 1024 * 1024

func startTestServer(t *testing.T) (pb.ShortenerServiceClient, func()) {
	t.Helper()

	authManager, err := auth.NewManager("test-auth-secret-16chars")
	if err != nil {
		t.Fatal(err)
	}

	service := handler.NewService(repository.NewInMemory())
	service.SetBaseURL("http://localhost:8080")
	t.Cleanup(service.Close)

	lis := bufconn.Listen(bufSize)
	srv := grpcserver.NewGRPCServer(service, authManager, nil)
	go func() {
		_ = srv.Serve(lis)
	}()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}

	return pb.NewShortenerServiceClient(conn), func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	}
}

func TestShortenExpandList(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	var header metadata.MD
	resp, err := client.ShortenURL(
		context.Background(),
		&pb.URLShortenRequest{Url: "https://praktikum.yandex.ru"},
		grpc.Header(&header),
	)
	if err != nil {
		t.Fatalf("ShortenURL: %v", err)
	}
	if resp.GetResult() == "" {
		t.Fatal("expected short url")
	}

	tokens := header.Get(auth.AuthorizationMetadata)
	if len(tokens) == 0 {
		t.Fatal("expected authorization header")
	}

	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(
		auth.AuthorizationMetadata, tokens[0],
	))

	expand, err := client.ExpandURL(ctx, &pb.URLExpandRequest{
		Id: lastPathSegment(resp.GetResult()),
	})
	if err != nil {
		t.Fatalf("ExpandURL: %v", err)
	}
	if expand.GetResult() != "https://praktikum.yandex.ru" {
		t.Fatalf("unexpected expand result: %s", expand.GetResult())
	}

	list, err := client.ListUserURLs(ctx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("ListUserURLs: %v", err)
	}
	if len(list.GetUrl()) != 1 {
		t.Fatalf("expected 1 url, got %d", len(list.GetUrl()))
	}
}

func TestShortenDuplicate(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	req := &pb.URLShortenRequest{Url: "https://ya.ru"}
	if _, err := client.ShortenURL(context.Background(), req); err != nil {
		t.Fatalf("first ShortenURL: %v", err)
	}

	_, err := client.ShortenURL(context.Background(), req)
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected AlreadyExists, got %v", err)
	}
}

func lastPathSegment(u string) string {
	for i := len(u) - 1; i >= 0; i-- {
		if u[i] == '/' {
			return u[i+1:]
		}
	}
	return u
}
