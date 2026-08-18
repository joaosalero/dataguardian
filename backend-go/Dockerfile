FROM golang:1.25-bookworm@sha256:e401dae1bf814e29204a8cb7915682e1780951e609ca0dd8865ee1937f510c48 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/dataguardian ./cmd/server

FROM debian:bookworm-slim@sha256:abd67ffcfa541b485a3dff59865ab629aa048a6c613e639d36e7456b0b229241 AS runtime

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
