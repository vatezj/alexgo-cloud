FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY . .
RUN go build -o /out/alexgo-server ./alexgo-server/cmd

FROM alpine:3.21
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /out/alexgo-server /app/alexgo-server
EXPOSE 8080
ENTRYPOINT ["/app/alexgo-server"]
