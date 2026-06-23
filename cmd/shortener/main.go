package main

import (
	"fmt"
	"net/http"

	"github.com/tkalexx/shorturl.git/internal/config"
	"github.com/tkalexx/shorturl.git/internal/handler"
)

func main() {
	// обрабатываем аргументы командной строки
	cfg := config.NewConfig()

	if err := run(cfg); err != nil {
		fmt.Printf("unexpected error: %v\n", err)
		return
	}
}

func run(cfg *config.Config) error {
	fmt.Printf("Server running on: %s\n", cfg.RunAddr)
	fmt.Printf("Base URL for shortened links: %s\n", cfg.BaseURL)

	return http.ListenAndServe(cfg.RunAddr, http.HandlerFunc(handler.Router))
}
