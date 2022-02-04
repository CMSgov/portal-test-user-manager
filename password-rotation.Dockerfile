FROM golang:1.16-alpine as build
COPY . /build
RUN cd /build; CGO_ENABLED=0 GOBIN=/bin/ go install .

FROM node:16-alpine
WORKDIR /home
RUN apk --update add ca-certificates
RUN npm install -g secure-spreadsheet
COPY --from=build /bin/portal-test-user-manager /bin/portal-test-user-manager
ENTRYPOINT ["/bin/portal-test-user-manager"]