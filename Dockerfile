# ---- build stage ----
FROM golang:1.27-alpine AS build
WORKDIR /src


# Copy the module files first so this layer is cached until dependencies change.
COPY go.mod go.sum ./
RUN go mod download


COPY .  .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/proxy ./cmd/proxy


# ---- run stage ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/proxy /proxy
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/proxy"]
