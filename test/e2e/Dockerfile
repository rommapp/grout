# Harness for driving grout's real UI headlessly.
#
# Everything runs inside this image, so the only thing the host needs is
# Docker. That makes the same command work on macOS and on CI runners.
FROM golang:1.25-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
      libsdl2-dev libsdl2-ttf-dev libsdl2-image-dev libsdl2-gfx-dev \
      xvfb x11-utils xdotool imagemagick openbox \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY . .

# The workspace points at a sibling gabagool checkout that is not in the build
# context, so build against the published module.
RUN rm -f go.work go.work.sum && GOWORK=off go build -o /grout ./app

ENV GROUT_BINARY=/grout
ENV DISPLAY=:99

# Development mode leaves the toolkit's keyboard mapping in place. Without it
# grout loads the firmware's controller mapping and ignores every keystroke.
ENV ENVIRONMENT=DEV

WORKDIR /src/test/e2e
