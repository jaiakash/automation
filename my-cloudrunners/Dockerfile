### Stage One
FROM golang:1.24-alpine AS build-golang

WORKDIR /app

COPY my-cloudrunners/ /app

# The WORKDIR is /app, and the source is in ./oci
RUN go build -o bin/cloudrunner-oci ./oci/main.go

### Final Stage
FROM alpine:latest

# Install any needed packages.
# ca-certificates is needed to trust TLS docertificates.
RUN apk --no-cache add ca-certificates

# Adjust the copy path to get the binary from the correct location
COPY --from=build-golang /app/bin/cloudrunner-oci /

ENTRYPOINT [ "/cloudrunner-oci" ]
