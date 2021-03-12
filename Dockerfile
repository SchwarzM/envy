FROM golang:1.16-alpine3.13 as builder
WORKDIR /go/src/github.com/schwarzm/envy/

COPY . .
RUN go build -o envy cmd/envy/main.go
FROM alpine:3.8 as envy
COPY --from=builder /go/src/github.com/schwarzm/envy/envy /
