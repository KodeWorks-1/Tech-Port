// Package assets embeds the UI (templates + static files) into the binary so
// single-binary deploys (Vercel's Go runtime, bare VPS) need no files on
// disk. Dev mode still reads from disk for live reload.
package assets

import "embed"

//go:embed views static
var FS embed.FS
