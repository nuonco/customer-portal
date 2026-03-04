package main

// Build styles (skips Tailwind if no CSS/templ files changed, runs both builds in parallel)
//go:generate ./scripts/build-css.sh

// Build templates
//go:generate go tool templ generate ./...
