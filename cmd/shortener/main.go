package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/tkalexx/shorturl.git/internal/config"
	"github.com/tkalexx/shorturl.git/internal/handler"
	"github.com/tkalexx/shorturl.git/internal/repository"
)

func main() {
	// обрабатываем аргументы командной строки
	cfg := config.NewConfig()

	if err := run(cfg); err != nil {
		log.Fatalf("unexpected error: %v", err)
	}
}

func run(cfg *config.Config) error {
	fmt.Printf("Server running on: %s\n", cfg.RunAddr)
	fmt.Printf("Base URL for shortened links: %s\n", cfg.BaseURL)

	repo := repository.NewInMemory()
	service := handler.NewService(repo)
	service.SetBaseURL(cfg.BaseURL)

	return http.ListenAndServe(cfg.RunAddr, handler.NewRouter(service))
}
