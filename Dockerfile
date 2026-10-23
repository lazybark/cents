FROM golang:1.27-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=1
RUN go build -mod=mod -trimpath -o /out/cents-cloud ./cmd/cloud \
 && go build -mod=mod -trimpath -o /out/cents-server ./cmd/server \
 && go build -mod=mod -trimpath -o /out/cents-bot ./cmd/bot

FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates sqlite3 \
 && rm -rf /var/lib/apt/lists/* \
 && mkdir -p /etc/cents /data
COPY --from=build /out/ /usr/local/bin/
COPY deploy/cloud.yaml.example /etc/cents/cloud.yml
WORKDIR /data
EXPOSE 8080
CMD ["cents-cloud", "--config", "/etc/cents/cloud.yml"]
