package repository

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

func BenchmarkInMemorySave(b *testing.B) {
	repo := NewInMemory()
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := fmt.Sprintf("id%08d", i)
		url := fmt.Sprintf("https://mem.example.com/%d", i)
		if err := repo.Save(ctx, id, url, "user"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFileSave(b *testing.B) {
	path := filepath.Join(b.TempDir(), "urls.json")
	repo, err := NewFileRepository(path)
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := fmt.Sprintf("id%08d", i)
		url := fmt.Sprintf("https://file.example.com/%d", i)
		if err := repo.Save(ctx, id, url, "user"); err != nil {
			b.Fatal(err)
		}
	}
}
