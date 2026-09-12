# ビルドコンテキストはリポジトリルート（compose参照）。
FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/task-dashboard .

# modernc.org/sqlite(cgo不要)を使っているため、静的リンクの最小イメージで動く。
# 既存のホスト側ボリューム(/docker/task-dashboard/data)がroot実行前提のパーミッションの
# ため、nonrootタグは使わない。
FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /out/task-dashboard ./

EXPOSE 8090

ENTRYPOINT ["/app/task-dashboard"]
