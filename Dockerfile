# syntax=docker/dockerfile:1

# ---- build the Go binary (cross-compiles natively for any target arch) ----
FROM --platform=$BUILDPLATFORM golang:1.26-trixie AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/seestaralign ./cmd/seestaralign

# ---- fetch ASTAP (headless CLI build) and a star database ----
FROM debian:trixie-slim AS astap
ARG TARGETARCH
# d50 is recommended for every Seestar (0.7°-2.4° fields). d80 = denser/bigger,
# d05 = small test build (may be too sparse for the S50), none = mount your own.
ARG ASTAP_DB=d50
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl unzip \
 && rm -rf /var/lib/apt/lists/*
RUN set -eux; \
    case "$TARGETARCH" in \
      amd64) arch=amd64 ;; arm64) arch=aarch64 ;; arm) arch=armhf ;; \
      *) echo "unsupported architecture: $TARGETARCH" >&2; exit 1 ;; \
    esac; \
    base=https://downloads.sourceforge.net/project/astap-program; \
    mkdir -p /opt/astap; \
    curl -fsSL --retry 5 -o /tmp/cli.zip "$base/linux_installer/astap_command-line_version_Linux_${arch}.zip"; \
    unzip -o /tmp/cli.zip -d /opt/astap; \
    chmod 755 /opt/astap/astap_cli; \
    if [ "$ASTAP_DB" != "none" ]; then \
      curl -fSL --retry 5 -o /tmp/db.zip "$base/star_databases/${ASTAP_DB}_star_database.zip"; \
      unzip -q -o /tmp/db.zip -d /opt/astap; \
      rm /tmp/db.zip; \
      find /opt/astap -mindepth 2 -name '*.1476' -exec mv -t /opt/astap {} +; \
      ls /opt/astap/*.1476 >/dev/null; \
    fi

# ---- runtime ----
FROM debian:trixie-slim
COPY --from=astap /opt/astap /opt/astap
COPY --from=build /out/seestaralign /usr/local/bin/seestaralign
RUN useradd --system --home-dir /data seestar \
 && mkdir -p /data && chown seestar /data
ENV SEESTARALIGN_CONFIG=/data/settings.json
USER seestar
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/seestaralign"]
CMD ["-listen", ":8080"]
