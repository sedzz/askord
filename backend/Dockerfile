# Imagen del backend (API + bot). Contexto de build: backend/
FROM golang:1.26-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=auto
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# Las migraciones van embebidas en el binario; no hace falta copiar nada más.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
