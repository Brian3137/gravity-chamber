# syntax=docker/dockerfile:1

# ---- build stage ------------------------------------------------------------
FROM golang:1.26-alpine AS build

WORKDIR /src

# Dependencias primero: esta capa se puede reutilizar
# mientras go.mod/go.sum no cambien.
COPY go.mod go.sum* ./
RUN go mod download

# Ahora copiamos el código fuente.
COPY . .

# Generamos un binario estático para Linux.
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o /bin/scouter ./cmd/scouter

# ---- runtime stage ----------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /bin/scouter /scouter

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/scouter"]
