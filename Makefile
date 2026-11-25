# Requires: templ (go install github.com/a-h/templ/cmd/templ@latest)
#           tailwindcss standalone CLI (e.g. `brew install tailwindcss`)
.PHONY: generate css run build dev test vendor

# Vendored front-end scripts. Bump a version here, then run `make vendor`
# and commit the updated files in static/vendor/.
HTMX_VERSION   ?= 2.0.11
ALPINE_VERSION ?= 3.17.5

generate:
	templ generate
vendor:
	mkdir -p static/vendor
	curl -fsSL -o static/vendor/htmx.min.js https://unpkg.com/htmx.org@$(HTMX_VERSION)/dist/htmx.min.js
	curl -fsSL -o static/vendor/alpine.min.js https://unpkg.com/alpinejs@$(ALPINE_VERSION)/dist/cdn.min.js
	@echo "vendored htmx $(HTMX_VERSION), alpine $(ALPINE_VERSION)"
css:
	tailwindcss -i assets/input.css -o static/app.css --minify
run: generate css
	go run .
build: generate css
	go build -o bin/hello-go .

test: generate
	go test -race ./...

# Live reload: templ watches .go/.templ (regenerate, restart, refresh browser
# via the proxy at http://localhost:8080) while Tailwind rebuilds the CSS.
dev:
	tailwindcss -i assets/input.css -o static/app.css --watch & \
	trap 'kill %1' EXIT; \
	PORT=8081 templ generate --watch \
		--proxy="http://localhost:8081" --proxyport=8080 --cmd="go run ."
