# 构建阶段
FROM golang:1.22-alpine AS builder
WORKDIR /workspace

# 复制依赖文件，利用 Docker 缓存层
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY main.go main.go
COPY api/ api/
COPY internal/ internal/

# 构建二进制文件（静态链接，支持多架构）
# TARGETARCH 由 docker buildx 自动设置；在 make docker-build 中也可通过 --build-arg 传入
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -a -o operator main.go

# 运行阶段 — 使用 distroless 最小化镜像
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/operator .
USER nonroot:nonroot

ENTRYPOINT ["/operator"]
