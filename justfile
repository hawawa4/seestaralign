bin := "bin/seestaralign"
ldflags := "-s -w"

export CGO_ENABLED := "0"

# list recipes
default:
    @just --list

# run locally with the simulator available; state lives in .dev/ so your real settings stay untouched
dev listen="127.0.0.1:8080":
    mkdir -p .dev
    go run ./cmd/seestaralign -listen {{listen}} -config .dev/settings.json -workdir .dev

# build for this machine
build:
    go build -trimpath -ldflags="{{ldflags}}" -o {{bin}} ./cmd/seestaralign

# go vet + all tests (no hardware or ASTAP needed; ~30 s)
test:
    go vet ./...
    go test ./...

linux-amd64:
    GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="{{ldflags}}" -o {{bin}}-linux-amd64 ./cmd/seestaralign

# 64-bit Raspberry Pi OS
linux-arm64:
    GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="{{ldflags}}" -o {{bin}}-linux-arm64 ./cmd/seestaralign

# build, then serve the release binary
run: build
    {{bin}} -listen 127.0.0.1:8080

# regenerate astropy reference values for the coordinate tests (needs uv)
reference:
    uv run -q tools/gen_reference.py > testdata/reference.json

clean:
    rm -rf bin .dev
