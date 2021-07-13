FROM golang:1.16 as build

ENV GO111MODULE=on

WORKDIR /go/release

ADD . .

RUN GOOS=linux CGO_ENABLED=0 GOARCH=amd64 go build -ldflags="-s -w" -installsuffix cgo -o app cmd/main.go

FROM scratch as prod

COPY --from=build /go/release/app /

ENTRYPOINT ["/app"]