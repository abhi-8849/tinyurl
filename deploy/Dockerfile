# Build the tinyurl image in two stages: compile in a full Go image, then copy only the binary into a tiny runtime image.

# ---- Stage 1: build ----
# Official Go image with the compiler. "AS build" names this stage so stage 2 can copy from it.
FROM golang:1.27 AS build

# All following commands run inside /src in the image.
WORKDIR /src

# Copy only the dependency files first. Docker caches this layer, so dependencies are not downloaded again unless these files change.
COPY go.mod go.sum ./

# Download the libraries listed in go.mod.
RUN go mod download

# Copy the rest of the source code.
COPY . .

# Compile a static binary: CGO_ENABLED=0 means no C libraries, so it runs on the minimal image below.
# -trimpath removes local file paths from the binary; -o names the output file.
RUN CGO_ENABLED=0 go build -trimpath -o /out/tinyurl ./cmd/tinyurl

# ---- Stage 2: run ----
# Distroless static image: no shell, no package manager, just enough to run a static binary. "nonroot" runs as an unprivileged user.
FROM gcr.io/distroless/static-debian12:nonroot

# Copy the compiled binary from the build stage.
COPY --from=build /out/tinyurl /tinyurl

# Document the port the server listens on (the default -addr is :8080). This does not publish the port by itself.
EXPOSE 8080

# Start the server. Flags are added by docker compose or Kubernetes through "command"/"args".
ENTRYPOINT ["/tinyurl"]
