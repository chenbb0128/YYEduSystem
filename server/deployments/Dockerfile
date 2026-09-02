# syntax=docker/dockerfile:1
ARG GO_VERSION=1.24.3

FROM golang:${GO_VERSION}-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN mkdir -p /src/data/uploads
ARG TARGET=api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/${TARGET}

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/app /app/app
COPY configs /app/configs
COPY database/migrations /app/database/migrations
COPY --from=build --chown=nonroot:nonroot /src/data/uploads /app/data/uploads
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/app"]
