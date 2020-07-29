FROM golang:1.14.6-alpine as build-env
WORKDIR /go/src/github.com/thesealion/wallet/
COPY . .
RUN CGO_ENABLED=0 go build -a -o goapp ./cmd/wallet

FROM alpine:3.7
WORKDIR /app
COPY --from=build-env /go/src/github.com/thesealion/wallet/goapp .
EXPOSE 8080
ENTRYPOINT ["./goapp"]
