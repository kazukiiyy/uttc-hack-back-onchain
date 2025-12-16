FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# ソースコードをすべてコピー
# (your_project/cmd/main.go や internal/ ディレクトリなど)
COPY . .

# アプリケーションをビルド
# -o /app/payment-service: バイナリ名を指定
# ./cmd/main.go: メインのエントリーポイントファイルを指定
# -ldflags -s -w: 不要なシンボル情報やデバッグ情報を削除し、最終バイナリサイズを削減
# -a -installsuffix cgo: 静的リンクでビルド（alpineのCライブラリ依存性を排除）
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o /app/payment-service ./main.go


# ----------------------------------------------------
# STAGE 2: 実行ステージ (Runtime Stage)
# ----------------------------------------------------
# 実行に必要な最小限の環境 (軽量なscratchまたはalpineが最適)
FROM alpine:latest

# 最終イメージの作業ディレクトリを設定
WORKDIR /root/

# ビルドステージで作成したバイナリをコピー
# --from=builder でビルドステージから /app/payment-service を取得
COPY --from=builder /app/payment-service .
# アプリケーションがリッスンするポートを公開
# Goコードで :8080 を使用しているため、ここでも 8080 を公開
EXPOSE 8080

# サーバーの起動 (Goバイナリを実行)
CMD ["./payment-service"]