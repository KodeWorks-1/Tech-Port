// Package migrations embeds the SQL migration files so the server can
// apply them at startup without a separate CLI.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
