package shorturl

import "embed"

// MigrationsFS содержит SQL-миграции, встроенные в бинарник.
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS
