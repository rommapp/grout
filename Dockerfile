FROM golang:1.24-bullseye

RUN apt-get update && apt-get install -y \
    libsdl2-dev \
    libsdl2-ttf-dev \
    libsdl2-image-dev \
    libsdl2-gfx-dev

WORKDIR /build

# Build argument to enable local gabagool development
ARG USE_LOCAL_GABAGOOL=false

# Copy source files
# Pattern works for both contexts: grout dir (go.mod) or parent dir (grout/go.mod)
COPY go*.mod go*.sum* go*.work* ./
COPY . .

# Move files to correct location if we're in a grout subdirectory from parent context
RUN if [ -d "grout" ] && [ "$USE_LOCAL_GABAGOOL" = "true" ]; then \
        echo "=== Reorganizing for workspace build ==="; \
        cd /; \
        mv /build /workspace-temp; \
        mkdir -p /workspace; \
        mv /workspace-temp /workspace/parent; \
        ln -s /workspace/parent/grout /build; \
        cd /build; \
    fi

# Set up workspace and download dependencies
RUN if [ "$USE_LOCAL_GABAGOOL" = "true" ]; then \
        if [ ! -f "go.work" ]; then \
            echo "ERROR: go.work not found!"; \
            echo "When USE_LOCAL_GABAGOOL=true, build context must be parent dir containing go.work"; \
            ls -la; \
            exit 1; \
        fi; \
        echo "=== Building with local gabagool workspace ==="; \
        cat go.work; \
        go work sync; \
    else \
        echo "=== Building with remote gabagool from go.mod ==="; \
        rm -f go.work go.work.sum; \
        GOWORK=off go mod download; \
    fi

# Build
RUN if [ "$USE_LOCAL_GABAGOOL" = "true" ]; then \
        go build -gcflags="all=-N -l" -v; \
    else \
        GOWORK=off go build -gcflags="all=-N -l" -v; \
    fi

CMD ["/bin/bash"]