package main

// Build styles
//go:generate go tool gotailwind -i ./src/vendor.css -o ./static/css/vendor.css --content './internal/views/vendorui/**/*.templ' --content './src/base.css'
//go:generate go tool gotailwind -i ./src/customer.css -o ./static/css/customer.css --content './internal/views/customerui/**/*.templ' --content './src/base.css'
//go:generate ./scripts/hash-assets.sh

// Build templates
//go:generate go tool templ generate ./...
