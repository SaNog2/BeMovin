// Package db holds the SQL migrations and queries.
package db

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
