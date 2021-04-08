package langs

import "embed"

// go:generate goi18n merge active.*.toml translate.*.toml

//go:embed active.*.toml
var LangFS embed.FS
