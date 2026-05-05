FROM golang:1.25-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/dataguardian ./cmd/server

FROM debian:bookworm-slim AS runtime

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates curl \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/dataguardian /app/dataguardian

RUN useradd --system --uid 10001 --create-home dataguardian \
  && mkdir -p /data/uploads \
  && chown -R dataguardian:dataguardian /data

USER dataguardian

EXPOSE 8000

CMD ["/app/dataguardian"]
