# Requires: templ (go install github.com/a-h/templ/cmd/templ@latest)
#           tailwindcss standalone CLI (e.g. `brew install tailwindcss`)
.PHONY: generate css run build dev
generate:
	templ generate
css:
	tailwindcss -i assets/input.css -o static/app.css --minify
run: generate css
	go run .
build: generate css
	go build -o bin/hello-go .

# Live reload: templ watches .go/.templ (regenerate, restart, refresh browser
# via the proxy at http://localhost:8080) while Tailwind rebuilds the CSS.
dev:
	tailwindcss -i assets/input.css -o static/app.css --watch & \
	trap 'kill %1' EXIT; \
	PORT=8081 templ generate --watch \
		--proxy="http://localhost:8081" --proxyport=8080 --cmd="go run ."
