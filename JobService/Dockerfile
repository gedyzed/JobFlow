FROM golang:1-alpine3.22 AS builder
WORKDIR /server
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0  go build -o server ./cmd/server/main.go
RUN CGO_ENABLED=0  go build -o relay ./cmd/relay/main.go
RUN CGO_ENABLED=0  go build -o worker ./cmd/worker/main.go

FROM alpine:3.22
WORKDIR /server
COPY --from=builder /server/server .
COPY --from=builder /server/relay .
COPY --from=builder /server/worker .
COPY --from=builder /server/worker .
EXPOSE 8080
ENTRYPOINT ["/server/server"]
