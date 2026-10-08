# 多阶段构建：BUILDPLATFORM 原生机交叉编译，避免多架构 QEMU 模拟
FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS build
WORKDIR /src
COPY . .
ARG TARGETARCH
RUN CGO_ENABLED=0 GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/vps-widget .

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/vps-widget /app/vps-widget
COPY public /app/public
ENV PORT=5555
ENV TZ=Asia/Shanghai
EXPOSE 5555
VOLUME ["/app/data"]
CMD ["/app/vps-widget"]
