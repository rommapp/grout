FROM golang:1.24-bullseye

RUN apt-get update && apt-get install -y \
    libsdl2-dev \
    libsdl2-ttf-dev \
    libsdl2-image-dev \
    libsdl2-gfx-dev \
    jq \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

ARG USE_LOCAL_GABAGOOL=false

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

ARG GITHUB_ACTIONS=false

RUN BUILD_TYPE="Dev"; \
    if [ "$GITHUB_ACTIONS" = "true" ]; then BUILD_TYPE="Release"; fi; \
    VERSION=$(jq -r '.version // "dev"' pak.json 2>/dev/null || echo "dev"); \
    GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
    BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ); \
    LDFLAGS="-X 'grout/version.Version=$VERSION' \
             -X 'grout/version.GitCommit=$GIT_COMMIT' \
             -X 'grout/version.BuildDate=$BUILD_DATE' \
             -X 'grout/version.BuildType=$BUILD_TYPE'"; \
    if [ "$USE_LOCAL_GABAGOOL" = "true" ]; then \
        go build -gcflags="all=-N -l" -ldflags "$LDFLAGS" -v -o grout ./app; \
    else \
        GOWORK=off go build -gcflags="all=-N -l" -ldflags "$LDFLAGS" -v -o grout ./app; \
    fi

CMD ["/bin/bash"]
