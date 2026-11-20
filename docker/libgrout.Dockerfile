FROM debian:bullseye

ARG TARGETARCH
ARG GO_VERSION
ARG GROUT_VERSION=dev

RUN printf '%s\n' \
        'deb http://archive.debian.org/debian bullseye main' \
        'deb http://archive.debian.org/debian-security bullseye-security main' \
        > /etc/apt/sources.list && \
    rm -f /etc/apt/sources.list.d/* && \
    apt-get -o Acquire::Check-Valid-Until=false update && \
    apt-get install -y --no-install-recommends gcc libc6-dev ca-certificates curl git && \
    rm -rf /var/lib/apt/lists/*

RUN case "$TARGETARCH" in \
        arm) GO_ARCH=armv6l ;; \
        *) GO_ARCH="$TARGETARCH" ;; \
    esac && \
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" | tar -C /usr/local -xz

ENV PATH=/usr/local/go/bin:$PATH
ENV GOWORK=off
ENV GOTOOLCHAIN=local

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN if [ "$TARGETARCH" = "arm" ]; then export GOARM=7; fi && \
    CGO_ENABLED=1 go build -buildmode=c-shared -trimpath \
        -ldflags "-s -w -X 'github.com/rommapp/grout/version.Version=${GROUT_VERSION}'" \
        -o /out/libgrout.so ./capi
