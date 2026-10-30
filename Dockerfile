FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
ARG TARGETOS TARGETARCH TARGETVARIANT
ARG VERSION COMMIT BUILD_DATE
WORKDIR /src

RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN GOARM="${TARGETVARIANT#v}" CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
  -ldflags "-s -w -X sss/internal/version.Version=${VERSION} -X sss/internal/version.Commit=${COMMIT} -X sss/internal/version.BuildDate=${BUILD_DATE}" \
  -o /out/sss .

RUN mkdir /out/config && printf 'LISTEN_ADDR=0.0.0.0:8080\nDATABASE_URL=/config/sss.db\nRUN_MIGRATIONS=1\n' > /out/config/sss.conf

FROM scratch
LABEL org.opencontainers.image.title=sss
LABEL org.opencontainers.image.description="Minimalist self-hosted feed reader"
LABEL org.opencontainers.image.vendor=pentago
LABEL org.opencontainers.image.licenses=Apache-2.0
LABEL org.opencontainers.image.url=https://github.com/pentago/sss
LABEL org.opencontainers.image.source=https://github.com/pentago/sss
LABEL org.opencontainers.image.documentation=https://github.com/pentago/sss#readme

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/sss /sss
COPY --from=build --chown=65534:65534 /out/config /config

EXPOSE 8080

VOLUME /config
USER 65534
HEALTHCHECK CMD ["/sss","-c","/config/sss.conf","-healthcheck","auto"]
ENTRYPOINT ["/sss","-c","/config/sss.conf"]
