package web

import "embed"

// Static contém os arquivos CSS e JS embutidos no binário.
//
//go:embed static
var Static embed.FS

// Templates contém os arquivos HTML embutidos no binário.
//
//go:embed templates
var Templates embed.FS
