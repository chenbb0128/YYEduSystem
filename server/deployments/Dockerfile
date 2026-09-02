# syntax=docker/dockerfile:1
ARG GO_VERSION=1.24.3

FROM golang:${GO_VERSION}-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN mkdir -p /src/data/uploads
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

ARG TARGET=api
RUN case "$TARGET" in \
      api|worker|migrate) cp "/out/${TARGET}" /out/app ;; \
      *) echo "Unsupported TARGET=${TARGET}" >&2; exit 64 ;; \
    esac

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/api /app/api
COPY --from=build /out/worker /app/worker
COPY --from=build /out/migrate /app/migrate
COPY --from=build /out/app /app/app
COPY configs /app/configs
COPY database/migrations /app/database/migrations
COPY --from=build --chown=nonroot:nonroot /src/data/uploads /app/data/uploads
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/app"]
