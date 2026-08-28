package schemas

import "embed"

// FS contains the canonical Atlas schemas shipped with the validator.
//
//go:embed *.schema.json
var FS embed.FS
