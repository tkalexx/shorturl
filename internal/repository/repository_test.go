package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInMemorySaveBatchAndPing(t *testing.T) {
	repo := NewInMemory()
	ctx := context.Background()

	if err := repo.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	mapping, err := repo.SaveBatch(ctx, []URLPair{
		{ID: "id1", URL: "https://a.example.com", UserID: "u1"},
		{ID: "id2", URL: "https://b.example.com", UserID: "u1"},
	})
	if err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}
	if mapping["https://a.example.com"] != "id1" || mapping["https://b.example.com"] != "id2" {
		t.Fatalf("unexpected mapping: %+v", mapping)
	}

	url, deleted, found := repo.Get(ctx, "id1")
	if !found || deleted || url != "https://a.example.com" {
		t.Fatalf("Get id1: url=%q deleted=%v found=%v", url, deleted, found)
	}
}

func TestInMemoryDuplicateAndDelete(t *testing.T) {
	repo := NewInMemory()
	ctx := context.Background()

	if err := repo.Save(ctx, "abc", "https://dup.example.com", "u1"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := repo.Save(ctx, "xyz", "https://dup.example.com", "u2"); err != ErrURLExists {
		t.Fatalf("expected ErrURLExists, got %v", err)
	}

	if err := repo.MarkDeleted(ctx, []string{"abc", "missing"}, "u1"); err != nil {
		t.Fatalf("MarkDeleted: %v", err)
	}
	_, deleted, found := repo.Get(ctx, "abc")
	if !found || !deleted {
		t.Fatalf("expected deleted url, found=%v deleted=%v", found, deleted)
	}

	urls, err := repo.GetByUserID(ctx, "u1")
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if len(urls) != 0 {
		t.Fatalf("expected no active urls, got %+v", urls)
	}
}

func TestFileRepositoryCRUD(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "urls.json")
	ctx := context.Background()

	repo, err := NewFileRepository(path)
	if err != nil {
		t.Fatalf("NewFileRepository: %v", err)
	}

	if err := repo.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	if err := repo.Save(ctx, "f1", "https://file1.example.com", "user-a"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := repo.Save(ctx, "f2", "https://file1.example.com", "user-b"); err != ErrURLExists {
		t.Fatalf("expected ErrURLExists, got %v", err)
	}

	mapping, err := repo.SaveBatch(ctx, []URLPair{
		{ID: "f3", URL: "https://file3.example.com", UserID: "user-a"},
	})
	if err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}
	if mapping["https://file3.example.com"] != "f3" {
		t.Fatalf("unexpected mapping: %+v", mapping)
	}

	url, deleted, found := repo.Get(ctx, "f1")
	if !found || deleted || url != "https://file1.example.com" {
		t.Fatalf("Get: url=%q deleted=%v found=%v", url, deleted, found)
	}

	id, ok := repo.FindByURL(ctx, "https://file1.example.com")
	if !ok || id != "f1" {
		t.Fatalf("FindByURL: id=%q ok=%v", id, ok)
	}

	urls, err := repo.GetByUserID(ctx, "user-a")
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if len(urls) != 2 {
		t.Fatalf("expected 2 urls, got %d", len(urls))
	}

	if err := repo.MarkDeleted(ctx, []string{"f1"}, "user-a"); err != nil {
		t.Fatalf("MarkDeleted: %v", err)
	}
	_, deleted, found = repo.Get(ctx, "f1")
	if !found || !deleted {
		t.Fatalf("expected deleted, found=%v deleted=%v", found, deleted)
	}

	// Перезагрузка из файла сохраняет данные.
	reloaded, err := NewFileRepository(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	_, deleted, found = reloaded.Get(ctx, "f3")
	if !found || deleted {
		t.Fatalf("reload Get f3: found=%v deleted=%v", found, deleted)
	}
	_, deleted, found = reloaded.Get(ctx, "f1")
	if !found || !deleted {
		t.Fatalf("reload Get f1: found=%v deleted=%v", found, deleted)
	}
}

func TestFileRepositoryEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatalf("write empty: %v", err)
	}

	repo, err := NewFileRepository(path)
	if err != nil {
		t.Fatalf("NewFileRepository empty: %v", err)
	}
	if _, _, found := repo.Get(context.Background(), "nope"); found {
		t.Fatal("expected not found")
	}
}
