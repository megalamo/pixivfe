module codeberg.org/pixivfe/pixivfe/v3

// We pin a Go version here to restrict language features
// (ref: https://go.dev/ref/mod#go-mod-file-go).
//
// When bumping the Go version, remember to update CI resources, Dockerfiles, and the READMEs.
go 1.25.0

require (
	// actor/message-passing library
	aidanwoods.dev/go-paseto v1.5.4
	// HTML scraping/parsing for pixivision pages and other HTML content
	github.com/PuerkitoBio/goquery v1.10.3

	// Server-side UI components/templating
	github.com/a-h/templ v0.3.960

	// YAML config load/save/print
	github.com/goccy/go-yaml v1.18.0

	// zstd and other codecs for transparent compression
	github.com/klauspost/compress v1.18.1

	// gettext runtime for i18n (catalog loading, translation, plural forms)
	github.com/leonelquinteros/gotext v1.7.2

	// TTY detection to toggle colored log output
	github.com/mattn/go-isatty v0.0.20

	// Structured logging
	github.com/rs/zerolog v1.34.0

	// Assertions for tests
	github.com/stretchr/testify v1.10.0

	// Easy querying of pixiv API JSON responses and validation - removal is TODO
	github.com/tidwall/gjson v1.18.0

	// HTML tokenizer and helpers (direct in ranking calendar; also indirect of goquery)
	golang.org/x/net v0.46.0

	// errgroup.Group for concurrent operations
	golang.org/x/sync v0.17.0

	// Locale/language matching and display names for i18n
	golang.org/x/text v0.30.0

	// Token bucket rate limiter for limiter middleware
	golang.org/x/time v0.14.0

	// go/packages for cmd/i18n_extract
	golang.org/x/tools v0.38.0
)

require github.com/mitchellh/go-server-timing v1.0.1

require (
	aidanwoods.dev/go-result v0.3.1 // indirect
	github.com/a-h/parse v0.0.0-20250122154542-74294addb73e // indirect
	github.com/andybalholm/brotli v1.2.0 // indirect
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/bombsimon/wsl/v5 v5.1.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cli/browser v1.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/felixge/httpsnoop v1.0.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/golang/gddo v0.0.0-20180823221919-9d8ff1c67be5 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/incu6us/goimports-reviser/v3 v3.9.1 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/natefinch/atomic v1.0.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/tidwall/match v1.2.0 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	golang.org/x/crypto v0.43.0 // indirect
	golang.org/x/exp v0.0.0-20250210185358-939b2ce775ac // indirect
	golang.org/x/mod v0.29.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	mvdan.cc/gofumpt v0.8.0 // indirect
)

// ref: https://tip.golang.org/doc/modules/managing-dependencies#tools
tool (
	github.com/a-h/templ/cmd/templ
	github.com/bombsimon/wsl/v5/cmd/wsl
	github.com/incu6us/goimports-reviser/v3
	mvdan.cc/gofumpt
)
