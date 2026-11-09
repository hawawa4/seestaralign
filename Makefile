BIN := bin/seestaralign
LDFLAGS := -s -w
export CGO_ENABLED := 0

.PHONY: build test linux-amd64 linux-arm64 run-sim reference clean

build:
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN) ./cmd/seestaralign

test:
	go vet ./...
	go test ./...

linux-amd64:
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN)-linux-amd64 ./cmd/seestaralign

linux-arm64:
	GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN)-linux-arm64 ./cmd/seestaralign

run: build
	$(BIN) -listen 127.0.0.1:8080

# regenerate astropy reference values for the coordinate tests (needs uv)
reference:
	uv run -q tools/gen_reference.py > testdata/reference.json

clean:
	rm -rf bin
