# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /vaultzap ./cmd/vaultzap

FROM gcr.io/distroless/static-debian12:nonroot
LABEL org.opencontainers.image.source="https://github.com/wallacepnts/vaultzap" \
      org.opencontainers.image.description="Local, browsable archive of exported WhatsApp conversations." \
      org.opencontainers.image.licenses="AGPL-3.0-only"
COPY --from=build /vaultzap /vaultzap
EXPOSE 8927
USER nonroot:nonroot
ENTRYPOINT ["/vaultzap"]
