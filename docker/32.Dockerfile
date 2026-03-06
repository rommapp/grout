FROM ghcr.io/brandonkowalski/quasimodo:latest

WORKDIR /build

ARG USE_LOCAL_GABAGOOL=false

# Copy dependency files first for layer caching
COPY go*.mod go*.sum* go*.work* ./

# For non-workspace builds, download dependencies as a separate cached layer
RUN if [ "$USE_LOCAL_GABAGOOL" != "true" ]; then \
        rm -f go.work go.work.sum; \
        GOWORK=off go mod download; \
    fi

# Copy source code (changes here don't invalidate the module download layer)
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

# Set up workspace dependencies (only for local gabagool builds)
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
    fi

ARG GITHUB_ACTIONS=false

# Build for ARM32 (Miyoo Mini Plus)
# Note: When using --platform=linux/arm/v7, Go automatically targets ARM32
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    BUILD_TYPE="Dev"; \
    if [ "$GITHUB_ACTIONS" = "true" ]; then BUILD_TYPE="Release"; fi; \
    VERSION=$(jq -r '.version // "dev"' pak.json 2>/dev/null || echo "dev"); \
    GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
    BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ); \
    LDFLAGS="-X 'grout/version.Version=$VERSION' \
             -X 'grout/version.GitCommit=$GIT_COMMIT' \
             -X 'grout/version.BuildDate=$BUILD_DATE' \
             -X 'grout/version.BuildType=$BUILD_TYPE'"; \
    echo "=== Building grout for Miyoo Mini Plus (ARM32) ==="; \
    if [ "$USE_LOCAL_GABAGOOL" = "true" ]; then \
        go build -ldflags "$LDFLAGS" -v -o grout ./app; \
    else \
        GOWORK=off go build -ldflags "$LDFLAGS" -v -o grout ./app; \
    fi

CMD ["/bin/bash"]
