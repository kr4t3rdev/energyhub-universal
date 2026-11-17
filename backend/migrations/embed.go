// Package migrations embeds the SQL files applied by cmd/migrate.
package migrations

import "embed"

// Files contains every SQL migration in this directory.
//
//go:embed *.sql
var Files embed.FS
