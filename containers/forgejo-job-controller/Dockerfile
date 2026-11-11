# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.26-alpine AS build
ARG TARGETOS=linux
ARG TARGETARCH
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/forgejo-job-controller ./cmd/forgejo-job-controller

FROM scratch
# CA bundle for an https Forgejo URL; the Kubernetes API uses the service account CA.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/forgejo-job-controller /forgejo-job-controller
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/forgejo-job-controller"]
