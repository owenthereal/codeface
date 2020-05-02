FROM golang:latest as builder

WORKDIR /go/src/codeface
RUN go get -u github.com/go-bindata/go-bindata/...
COPY . .
RUN make install

FROM heroku/heroku:20
MAINTAINER Owen Ou

RUN useradd -m -s /usr/bin/bash dyno
USER dyno
WORKDIR /home/dyno

COPY --from=builder /go/bin/cf /usr/bin/cf

ENTRYPOINT ["cf"]
CMD ["server"]
