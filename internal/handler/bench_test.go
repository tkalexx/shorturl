package handler

import (
	"context"
	"fmt"
	"testing"

	"github.com/tkalexx/shorturl.git/internal/repository"
)

func BenchmarkShorten(b *testing.B) {
	repo := repository.NewInMemory()
	service := NewService(repo)
	service.SetBaseURL("http://localhost:8080")
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		url := fmt.Sprintf("https://benchmark.example.com/path/%d", i)
		if _, _, err := service.Shorten(ctx, url, "bench-user"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGet(b *testing.B) {
	repo := repository.NewInMemory()
	service := NewService(repo)
	service.SetBaseURL("http://localhost:8080")
	ctx := context.Background()

	ids := make([]string, 1000)
	for i := 0; i < len(ids); i++ {
		short, _, err := service.Shorten(ctx, fmt.Sprintf("https://get.example.com/%d", i), "bench-user")
		if err != nil {
			b.Fatal(err)
		}
		ids[i] = short[len(service.baseURL)+1:]
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := service.Get(ctx, ids[i%len(ids)]); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkShortenBatch(b *testing.B) {
	repo := repository.NewInMemory()
	service := NewService(repo)
	service.SetBaseURL("http://localhost:8080")
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		items := []BatchItem{
			{CorrelationID: "1", OriginalURL: fmt.Sprintf("https://batch.example.com/a/%d", i)},
			{CorrelationID: "2", OriginalURL: fmt.Sprintf("https://batch.example.com/b/%d", i)},
			{CorrelationID: "3", OriginalURL: fmt.Sprintf("https://batch.example.com/c/%d", i)},
		}
		if _, err := service.ShortenBatch(ctx, items, "bench-user"); err != nil {
			b.Fatal(err)
		}
	}
}
