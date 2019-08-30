# ref: https://github.com/percona/mongodb_exporter/blob/master/Dockerfile

FROM golang:alpine as builder

RUN apk --no-cache add curl git make perl

RUN git clone https://github.com/percona/mongodb_exporter.git /go/src/github.com/percona/mongodb_exporter \
    && cd /go/src/github.com/percona/mongodb_exporter \
    && git checkout master

WORKDIR /go/src/github.com/percona/mongodb_exporter

RUN make build

FROM       alpine
EXPOSE     9216

RUN apk add --update ca-certificates
COPY --from=builder /go/src/github.com/percona/mongodb_exporter/bin/mongodb_exporter /bin/mongodb_exporter

ENTRYPOINT [ "/bin/mongodb_exporter" ]
