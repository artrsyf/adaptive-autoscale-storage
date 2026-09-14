FROM golang:1.25.7-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/storage ./cmd/storage && CGO_ENABLED=0 go build -trimpath -o /out/load ./cmd/load

FROM build AS test
ENTRYPOINT ["go", "test", "-race", "-count=1", "./..."]

FROM debian:bookworm-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/ /usr/local/bin/
USER 65532:65532
EXPOSE 8080 9091
ENTRYPOINT ["storage"]
