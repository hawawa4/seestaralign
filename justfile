bin := "bin/seestaralign"
ldflags := "-s -w"

export CGO_ENABLED := "0"

# list recipes
default:
    @just --list

# install frontend dependencies (only when package-lock.json changed)
_deps:
    cd web && [ node_modules/.package-lock.json -nt package-lock.json ] || npm ci --no-audit --no-fund

# build the frontend into internal/web/dist, which the Go binary embeds
web: _deps
    cd web && npm run build

# develop with hot reload: Go API on `api`, Vite UI on http://localhost:5173. Settings live in .dev/
dev api="127.0.0.1:8080": _deps
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p .dev
    trap 'kill 0' EXIT
    go run ./cmd/seestaralign -listen {{api}} -config .dev/settings.json -workdir .dev &
    cd web && API_URL=http://{{api}} npm run dev

# build for this machine
build: web
    go build -trimpath -ldflags="{{ldflags}}" -o {{bin}} ./cmd/seestaralign

# frontend type-check and unit tests, then go vet and all Go tests (~30 s, no hardware or ASTAP needed)
test: _deps
    cd web && npm run check && npm test
    go vet ./...
    go test ./...

linux-amd64: web
    GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="{{ldflags}}" -o {{bin}}-linux-amd64 ./cmd/seestaralign

# 64-bit Raspberry Pi OS
linux-arm64: web
    GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="{{ldflags}}" -o {{bin}}-linux-arm64 ./cmd/seestaralign

# build, then serve the release binary
run: build
    {{bin}} -listen 127.0.0.1:8080

# regenerate astropy reference values for the coordinate tests (needs uv)
reference:
    uv run -q tools/gen_reference.py > testdata/reference.json

clean:
    rm -rf bin .dev web/node_modules
    node web/scripts/clean-dist.mjs
