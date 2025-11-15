FROM ghcr.io/onionui/miyoomini-toolchain:latest AS builder

# Install Go in the Miyoo toolchain container
RUN wget -O go.tar.gz https://go.dev/dl/go1.24.1.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go.tar.gz && \
    rm go.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOROOT="/usr/local/go"

# Find the actual cross-compiler in the Miyoo toolchain
RUN echo "=== Finding Miyoo Cross-Compiler ===" && \
    find /opt -name "*gcc*" -type f 2>/dev/null | grep arm | head -10 && \
    find /usr -name "*gcc*" -type f 2>/dev/null | grep arm | head -10 && \
    find / -name "arm-*-gcc" -type f 2>/dev/null | head -10

# Set PATH to include toolchain locations
ENV PATH="/opt/miyoomini-toolchain/usr/bin:/opt/miyoomini-toolchain/bin:/usr/local/bin:${PATH}"

# Try to find and test the compiler
RUN echo "=== Testing Cross-Compiler Paths ===" && \
    which arm-linux-gnueabihf-gcc || echo "arm-linux-gnueabihf-gcc not found" && \
    ls -la /opt/*/bin/*gcc* 2>/dev/null || echo "No gcc in /opt/*/bin/" && \
    ls -la /opt/*/usr/bin/*gcc* 2>/dev/null || echo "No gcc in /opt/*/usr/bin/"

# Fix Debian Buster repositories (moved to archive)
RUN sed -i 's|http://deb.debian.org/debian|http://archive.debian.org/debian|g' /etc/apt/sources.list && \
    sed -i 's|http://security.debian.org/debian-security|http://archive.debian.org/debian-security|g' /etc/apt/sources.list && \
    sed -i '/buster-updates/d' /etc/apt/sources.list && \
    echo "Acquire::Check-Valid-Until false;" > /etc/apt/apt.conf.d/99no-check-valid-until

# Install basic development tools
RUN apt-get update && apt-get install -y \
    pkg-config \
    wget \
    build-essential \
    autoconf \
    automake \
    libtool \
    cmake \
    git \
    nasm

# Find the actual compiler and set it up
RUN ACTUAL_GCC=$(find /opt -name "*arm*gcc" -type f 2>/dev/null | head -1) && \
    echo "Found compiler: $ACTUAL_GCC" && \
    if [ -n "$ACTUAL_GCC" ]; then \
        ln -sf "$ACTUAL_GCC" /usr/local/bin/arm-linux-gnueabihf-gcc; \
        ACTUAL_GPP=$(echo $ACTUAL_GCC | sed 's/gcc$/g++/'); \
        if [ -f "$ACTUAL_GPP" ]; then \
            ln -sf "$ACTUAL_GPP" /usr/local/bin/arm-linux-gnueabihf-g++; \
        fi; \
    else \
        echo "No ARM cross-compiler found!"; \
        exit 1; \
    fi

# Verify compiler is now working
RUN echo "=== Verifying Cross-Compiler ===" && \
    which arm-linux-gnueabihf-gcc && \
    arm-linux-gnueabihf-gcc --version

# Set up cross-compilation environment for SDL2 build
ENV CROSS_COMPILE=arm-linux-gnueabihf-
ENV CC=arm-linux-gnueabihf-gcc
ENV CXX=arm-linux-gnueabihf-g++
ENV AR=arm-linux-gnueabihf-ar
ENV STRIP=arm-linux-gnueabihf-strip
ENV RANLIB=arm-linux-gnueabihf-ranlib
ENV SYSROOT=/usr/arm-linux-gnueabihf

# Create sysroot directory
RUN mkdir -p $SYSROOT

WORKDIR /tmp

# Build minimal SDL2 from source for Miyoo Mini Plus
RUN echo "=== Building minimal SDL2 from source for Miyoo Mini Plus ===" && \
    rm -f SDL2-2.30.9.tar.gz* && \
    wget https://github.com/libsdl-org/SDL/releases/download/release-2.30.9/SDL2-2.30.9.tar.gz && \
    rm -rf SDL2-2.30.9 && \
    tar -xzf SDL2-2.30.9.tar.gz && \
    cd SDL2-2.30.9 && \
    ./configure \
        --host=arm-linux-gnueabihf \
        --prefix=$SYSROOT/usr \
        --disable-static \
        --enable-shared \
        --enable-video-fbcon \
        --enable-video-dummy \
        --disable-video-x11 \
        --disable-video-wayland \
        --disable-pulseaudio \
        --disable-alsa \
        --enable-audio-dummy \
        --disable-haptic \
        --enable-joystick \
        --disable-power \
        --enable-filesystem \
        --enable-timers \
        --enable-file \
        --enable-loadso \
        --enable-cpuinfo \
        --disable-assembly \
        --enable-threads \
        --enable-atomic \
        --enable-events \
        --enable-video \
        --enable-render \
        --enable-video-opengl=no \
        --enable-video-opengles=no \
        --enable-video-opengles1=no \
        --enable-video-opengles2=no && \
    make -j$(nproc) && \
    make install

# Build SDL2_ttf
RUN echo "=== Building SDL2_ttf ===" && \
    wget https://github.com/libsdl-org/SDL_ttf/releases/download/release-2.22.0/SDL2_ttf-2.22.0.tar.gz && \
    tar -xzf SDL2_ttf-2.22.0.tar.gz && \
    cd SDL2_ttf-2.22.0 && \
    ./configure \
        --host=arm-linux-gnueabihf \
        --prefix=$SYSROOT/usr \
        --disable-static \
        --enable-shared \
        --with-sdl-prefix=$SYSROOT/usr && \
    make -j$(nproc) && \
    make install

# Build SDL2_image (minimal - just PNG support)
RUN echo "=== Building SDL2_image ===" && \
    wget https://github.com/libsdl-org/SDL_image/releases/download/release-2.8.2/SDL2_image-2.8.2.tar.gz && \
    tar -xzf SDL2_image-2.8.2.tar.gz && \
    cd SDL2_image-2.8.2 && \
    ./configure \
        --host=arm-linux-gnueabihf \
        --prefix=$SYSROOT/usr \
        --disable-static \
        --enable-shared \
        --enable-png \
        --disable-jpg \
        --disable-tif \
        --disable-webp \
        --with-sdl-prefix=$SYSROOT/usr && \
    make -j$(nproc) && \
    make install

# Build SDL2_gfx
RUN echo "=== Building SDL2_gfx ===" && \
    wget https://www.ferzkopp.net/Software/SDL2_gfx/SDL2_gfx-1.0.4.tar.gz && \
    tar -xzf SDL2_gfx-1.0.4.tar.gz && \
    cd SDL2_gfx-1.0.4 && \
    ./configure \
        --host=arm-linux-gnueabihf \
        --prefix=$SYSROOT/usr \
        --disable-static \
        --enable-shared \
        --disable-mmx \
        --with-sdl-prefix=$SYSROOT/usr && \
    make -j$(nproc) && \
    make install

# Create SDL2_gfx pkg-config file manually if it doesn't exist
RUN mkdir -p $SYSROOT/usr/lib/pkgconfig
RUN echo 'prefix=/usr/arm-linux-gnueabihf/usr' > $SYSROOT/usr/lib/pkgconfig/SDL2_gfx.pc && \
    echo 'exec_prefix=${prefix}' >> $SYSROOT/usr/lib/pkgconfig/SDL2_gfx.pc && \
    echo 'libdir=${exec_prefix}/lib' >> $SYSROOT/usr/lib/pkgconfig/SDL2_gfx.pc && \
    echo 'includedir=${prefix}/include' >> $SYSROOT/usr/lib/pkgconfig/SDL2_gfx.pc && \
    echo '' >> $SYSROOT/usr/lib/pkgconfig/SDL2_gfx.pc && \
    echo 'Name: SDL2_gfx' >> $SYSROOT/usr/lib/pkgconfig/SDL2_gfx.pc && \
    echo 'Description: drawing and graphical effects extension for SDL2' >> $SYSROOT/usr/lib/pkgconfig/SDL2_gfx.pc && \
    echo 'Version: 1.0.4' >> $SYSROOT/usr/lib/pkgconfig/SDL2_gfx.pc && \
    echo 'Requires: sdl2 >= 2.0.0' >> $SYSROOT/usr/lib/pkgconfig/SDL2_gfx.pc && \
    echo 'Conflicts:' >> $SYSROOT/usr/lib/pkgconfig/SDL2_gfx.pc && \
    echo 'Libs: -L${libdir} -lSDL2_gfx' >> $SYSROOT/usr/lib/pkgconfig/SDL2_gfx.pc && \
    echo 'Cflags: -I${includedir}/SDL2' >> $SYSROOT/usr/lib/pkgconfig/SDL2_gfx.pc

# Set up cross-compilation environment for Go
ENV GOOS=linux
ENV GOARCH=arm
ENV GOARM=7
ENV CGO_ENABLED=1
ENV PKG_CONFIG_PATH=$SYSROOT/usr/lib/pkgconfig
ENV CGO_CFLAGS="-I$SYSROOT/usr/include -I$SYSROOT/usr/include/SDL2"
ENV CGO_LDFLAGS="-L$SYSROOT/usr/lib -Wl,-rpath-link=$SYSROOT/usr/lib"

WORKDIR /build

COPY go.mod go.sum* ./
RUN GOWORK=off go mod download

COPY . .
RUN mkdir -p /build/bin /build/lib/miyoo

# Verify SDL2 setup before building
RUN echo "=== Verifying SDL2 Installation ===" && \
    ls -la $SYSROOT/usr/include/SDL2/ && \
    ls -la $SYSROOT/usr/lib/libSDL2* && \
    echo "=== Checking pkg-config files ===" && \
    ls -la $SYSROOT/usr/lib/pkgconfig/ && \
    pkg-config --modversion sdl2 --define-prefix --prefix=$SYSROOT/usr || echo "sdl2 pkg-config check failed" && \
    pkg-config --modversion SDL2_gfx --define-prefix --prefix=$SYSROOT/usr || echo "SDL2_gfx pkg-config check failed"

# Build for Miyoo Mini Plus
RUN echo "=== Building for Miyoo Mini Plus ===" && \
    GOWORK=off go build -gcflags="all=-N -l" -v -o /build/bin/grout-miyoo

# Copy the ARM SDL2 libraries
RUN echo "=== Copying ARM SDL2 libraries ===" && \
    cp $SYSROOT/usr/lib/libSDL2-2.0.so* /build/lib/miyoo/ && \
    cp $SYSROOT/usr/lib/libSDL2_ttf-2.0.so* /build/lib/miyoo/ && \
    cp $SYSROOT/usr/lib/libSDL2_image-2.0.so* /build/lib/miyoo/ && \
    cp $SYSROOT/usr/lib/libSDL2_gfx-1.0.so* /build/lib/miyoo/ && \
    echo "=== ARM SDL2 Libraries Copied ===" && \
    ls -la /build/lib/miyoo/

FROM ghcr.io/onionui/miyoomini-toolchain:latest

WORKDIR /app

COPY --from=builder /build/bin/ /app/bin/
COPY --from=builder /build/lib/ /app/lib/

# Final analysis
RUN echo "=== Miyoo Mini Plus Build Complete ===" && \
    echo "Built for Miyoo Mini Plus:" && \
    ls -la /app/bin/ && \
    file /app/bin/* && \
    echo "=== Dependency Analysis ===" && \
    readelf -d /app/bin/grout-miyoo | grep NEEDED && \
    echo "=== Available SDL2 Libraries ===" && \
    ls -la /app/lib/miyoo/ && \
    echo "=== Toolchain Info ===" && \
    find /opt -name "*gcc*" -type f | grep arm | head -1 | xargs basename

CMD ["/bin/bash"]
