FROM golang:1.14 as build
COPY . /build
RUN cd /build; CGO_ENABLED=0 GOBIN=/bin/ go install ./cmd/test-script

FROM scratch
COPY --from=build /bin/test-script /bin/test-script
ENTRYPOINT ["/bin/test-script"]
