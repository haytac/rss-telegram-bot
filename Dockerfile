FROM golang:1.24.3-alpine AS builder

WORKDIR /app

# Install build tools including GCC
# 'build-base' includes gcc, make, etc.
# 'sqlite-dev' might be needed if the CGO sqlite driver has system dependencies,
# though often mattn/go-sqlite3 bundles what it needs or builds statically.
# Start with build-base.
RUN apk add --no-cache build-base sqlite-dev # <--- ADD THIS LINE

# Copy go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the application
# CGO_ENABLED=1 is important for SQLite static linking and smaller images if not using system libs
# Using -tags sqlite_omit_load_extension to potentially reduce attack surface if extensions aren't needed.
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -tags="sqlite_omit_load_extension" -o /rss-telegram-bot cmd/rss-telegram-bot/main.go

# --- Final Stage ---
FROM alpine:latest

# Add necessary certificates for HTTPS requests and timezone data
RUN apk --no-cache add ca-certificates tzdata

# For SQLite, CGO_ENABLED=1 build might need libc.
# Alpine's base image has musl libc, which mattn/go-sqlite3 should work with.
# The sqlite-dev from the builder stage is NOT needed in the final image
# if the Go binary is statically linked or bundles the necessary SQLite parts.
# However, the final image MIGHT need `sqlite-libs` if your Go binary isn't fully static regarding SQLite.
# Let's try without it first. If you get runtime errors about missing libsqlite3.so, add:
# RUN apk --no-cache add sqlite-libs

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /rss-telegram-bot /app/rss-telegram-bot

# Copy migrations (if you want to run them from container, or bake into app)
# Assuming migrations path is now relative to WORKDIR /app
COPY --from=builder /app/internal/database/migrations ./internal/database/migrations

# Copy example config (optional, user should mount their own)
COPY config.yml.example /app/config.yml.example

# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

# Data volume for database and logs
VOLUME /app/data

# Default command (can be overridden)
ENTRYPOINT ["/app/rss-telegram-bot"]
CMD ["run", "--config", "/app/config.yml"]