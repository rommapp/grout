FROM --platform=linux/amd64 debian:bookworm

ARG GO_VERSION
ARG NDK_VERSION=r27d
ARG GROUT_VERSION=dev

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl git unzip cmake make && \
    rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | tar -C /usr/local -xz

RUN curl -fsSL -o /tmp/ndk.zip "https://dl.google.com/android/repository/android-ndk-${NDK_VERSION}-linux.zip" && \
    unzip -q /tmp/ndk.zip -d /opt && \
    mv "/opt/android-ndk-${NDK_VERSION}" /opt/ndk && \
    rm /tmp/ndk.zip

ENV NDK_BIN=/opt/ndk/toolchains/llvm/prebuilt/linux-x86_64/bin
ENV PATH=/usr/local/go/bin:/opt/ndk/toolchains/llvm/prebuilt/linux-x86_64/bin:$PATH
ENV GOWORK=off
ENV GOTOOLCHAIN=local

WORKDIR /build

COPY . .

RUN go mod download

RUN scripts/build-sigil.sh -DCMAKE_TOOLCHAIN_FILE=/opt/ndk/build/cmake/android.toolchain.cmake -DANDROID_PLATFORM=android-28 -DANDROID_ABI=arm64-v8a

RUN CGO_ENABLED=1 GOOS=android GOARCH=arm64 CC=aarch64-linux-android28-clang \
    go build -buildmode=c-shared -trimpath \
        -ldflags "-s -w -X 'github.com/rommapp/grout/version.Version=${GROUT_VERSION}' -extldflags '-Wl,-z,max-page-size=16384 -Wl,--exclude-libs,ALL'" \
        -o /out/arm64-v8a/libgrout.so ./capi

RUN scripts/build-sigil.sh -DCMAKE_TOOLCHAIN_FILE=/opt/ndk/build/cmake/android.toolchain.cmake -DANDROID_PLATFORM=android-28 -DANDROID_ABI=armeabi-v7a

RUN CGO_ENABLED=1 GOOS=android GOARCH=arm GOARM=7 CC=armv7a-linux-androideabi28-clang \
    go build -buildmode=c-shared -trimpath \
        -ldflags "-s -w -X 'github.com/rommapp/grout/version.Version=${GROUT_VERSION}' -extldflags '-Wl,-z,max-page-size=16384 -Wl,--exclude-libs,ALL'" \
        -o /out/armeabi-v7a/libgrout.so ./capi

RUN llvm-readelf -lW /out/arm64-v8a/libgrout.so | \
    awk '$1 == "LOAD" { n++; if ($NF != "0x4000") { bad = 1; print "bad LOAD alignment: " $NF } } END { if (n == 0 || bad) exit 1 }'
