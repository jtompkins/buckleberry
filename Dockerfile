# ---- build stage ----
FROM golang:1.26-bookworm AS build

WORKDIR /src

# Download dependencies first so this layer is cached across source changes.
COPY go.mod go.sum ./
RUN go mod download

# Build a fully static, CGO-free binary (the pure-Go sqlite driver makes this
# possible), so it can run on the minimal distroless/static base.
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /buckleberry .

# Empty dir to seed the SQLite volume, so the nonroot user can write to it.
RUN mkdir /data

# ---- runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /buckleberry /buckleberry
COPY --from=build --chown=65532:65532 /data /data

# Persist the database outside the image. In Coolify, mount a volume at /data.
ENV DB_PATH=/data/buckleberry.db
ENV PORT=8080
EXPOSE 8080
VOLUME ["/data"]

ENTRYPOINT ["/buckleberry"]
