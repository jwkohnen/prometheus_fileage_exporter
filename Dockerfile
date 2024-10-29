FROM	golang:1.23.2 as build
ENV	GO111MODULES=on CGO_ENABLED=0
WORKDIR	/nogopath
COPY	go.mod go.sum ./
RUN	go mod download
RUN	go mod verify
COPY	*.go ./
COPY	exporter exporter
RUN	go vet ./...
RUN	go install --mod=readonly

FROM	scratch
COPY	--from=build /go/bin/prometheus_fileage_exporter /
EXPOSE	9104
STOPSIGNAL	SIGINT
ENTRYPOINT	[ "/prometheus_fileage_exporter" ]
CMD	[ "--help" ]
