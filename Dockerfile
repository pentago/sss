FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
ARG TARGETOS TARGETARCH TARGETVARIANT
ARG VERSION COMMIT BUILD_DATE
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH \
    GOARM=${TARGETVARIANT#v} \
    go build -trimpath \
      -ldflags "-s -w \
      -X sss/internal/version.Version=${VERSION} \
      -X sss/internal/version.Commit=${COMMIT} \
      -X sss/internal/version.BuildDate=${BUILD_DATE}" \
      -o /out/sss .

RUN mkdir /out/config && cat <<'EOF' > /out/config/sss.conf
LISTEN_ADDR=0.0.0.0:8080
DATABASE_PATH=/config/sss.db
RUN_MIGRATIONS=1
EOF

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build --chown=65534:65534 /out/config /config
COPY --from=build /out/sss /sss

USER 65534
VOLUME /config
EXPOSE 8080
HEALTHCHECK CMD ["/sss","-c","/config/sss.conf","-healthcheck","auto"]
ENTRYPOINT ["/sss","-c","/config/sss.conf"]
