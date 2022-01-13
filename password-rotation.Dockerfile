FROM alpine:latest as certs
RUN apk --update add ca-certificates

FROM golang:1.16-alpine as build
COPY . /build
RUN cd /build; CGO_ENABLED=0 GOBIN=/bin/ go install .

FROM scratch
COPY --from=build /bin/portal-test-user-manager /bin/portal-test-user-manager
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=certs /tmp /tmp
ENTRYPOINT ["/bin/portal-test-user-manager"]
