.PHONY: generate run build dev
generate:
	templ generate
run: generate
	go run .
build: generate
	go build -o bin/hello-go .

# Live reload: regenerates templates, restarts the server on .go/.templ/.css
# changes, and refreshes the browser via a proxy at http://localhost:7331.
dev:
	PORT=8081 templ generate --watch \
		--watch-pattern '(.+\.go$$)|(.+\.templ$$)|(.+\.css$$)' \
		--proxy="http://localhost:8081" \
		--proxyport=8080 \
		--cmd="go run ."
