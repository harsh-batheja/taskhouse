FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/taskhouse-server ./cmd/server
RUN CGO_ENABLED=0 go build -o /bin/task ./cmd/task

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/taskhouse-server /usr/local/bin/taskhouse-server
COPY --from=builder /bin/task /usr/local/bin/task
EXPOSE 8080
ENTRYPOINT ["taskhouse-server"]
