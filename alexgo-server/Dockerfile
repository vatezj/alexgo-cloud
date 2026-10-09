FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY . .
# 双二进制（micro 形态）：system-server（默认 ENTRYPOINT）+ member-server（compose/k8s 用 command 覆盖启动）
RUN go build -o /out/alexgo-server ./alexgo-server/cmd && \
    go build -o /out/member-server ./modules/member/cmd

FROM alpine:3.21
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /out/alexgo-server /app/alexgo-server
COPY --from=builder /out/member-server /app/member-server
EXPOSE 8080
ENTRYPOINT ["/app/alexgo-server"]
