# ref: https://github.com/percona/mongodb_exporter/blob/master/Dockerfile

FROM alpine as builder

WORKDIR /percona/

RUN wget https://github.com/percona/mongodb_exporter/releases/download/v0.8.0/mongodb_exporter-0.8.0.linux-amd64.tar.gz
RUN tar -zxvf mongodb_exporter-0.8.0.linux-amd64.tar.gz

FROM       alpine
EXPOSE     9216

RUN apk add --update ca-certificates
COPY --from=builder /percona/mongodb_exporter /bin/mongodb_exporter

ENTRYPOINT  [ "/bin/mongodb_exporter" ]
