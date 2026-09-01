FROM docker.io/library/node:24.20.0-bookworm-slim@sha256:6642ef280aebc09c4541bee0b15c9f89f0f3f3c247ddee79ae1d37eddfdcbbaa AS frontend
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM docker.io/library/golang:1.26.7-bookworm@sha256:659cc38c1a394eeb4dd7e31fff6df128bd33444dcc7afd70e3bed5225749dbc0 AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN GOTOOLCHAIN=local go mod download
COPY --from=frontend /src /src
ARG VERSION
RUN test -n "$VERSION" \
    && printf '%s\n' "$VERSION" | grep -Eq '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$' \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=local go build -trimpath -buildvcs=false -ldflags "-s -w -X github.com/yangphere/leanote/app/service.BuildVersion=$VERSION" -o /out/leanote ./cmd/leanote

FROM docker.io/library/debian:bookworm-slim@sha256:5ae3c39ebd15e229dcedd5cee596b2497182493d41ff162e824ba13fc1b2b867
ARG VERSION
ARG REVISION=unknown
ARG SOURCE_DATE_EPOCH=0
LABEL org.opencontainers.image.version="$VERSION" \
      org.opencontainers.image.revision="$REVISION" \
      org.opencontainers.image.created="$SOURCE_DATE_EPOCH" \
      org.opencontainers.image.source="https://github.com/yangphere/leanote"
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates fontconfig fonts-dejavu wkhtmltopdf=0.12.6-2+b1 \
    && rm -rf /var/lib/apt/lists/* \
    && ln -s /usr/bin/wkhtmltopdf /usr/local/bin/wkhtmltopdf \
    && groupadd --gid 10001 leanote \
    && useradd --uid 10001 --gid 10001 --create-home --shell /usr/sbin/nologin leanote \
    && mkdir -p /app/bin /app/app /app/messages /app/public /app/files /app/public/upload /etc/leanote \
    && chown -R 10001:10001 /app /etc/leanote
WORKDIR /app
COPY --from=backend /out/leanote /app/bin/leanote
COPY --from=frontend /src/app/views /app/app/views
COPY --from=frontend /src/messages /app/messages
COPY --from=frontend /src/public /app/public
COPY conf/app.conf-default /app/conf/app.conf-default
COPY conf/routes /app/conf/routes
RUN chmod 0755 /app/bin/leanote && chown -R 10001:10001 /app
VOLUME ["/app/files", "/app/public/upload"]
USER 10001:10001
ENTRYPOINT ["/app/bin/leanote", "-conf", "/etc/leanote/app.conf", "-runMode", "prod"]
