package docs

import "embed"

//go:embed openapi.yaml portal.html terms.html
var files embed.FS
