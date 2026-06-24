# --- build stage ---
FROM golang:1.25-alpine AS build

WORKDIR /src

# Cache de dependencias.
COPY go.mod go.sum ./
RUN go mod download

# Código y compilación estática (binario sin libc, ideal para distroless).
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/server

# --- runtime stage ---
# Imagen mínima, sin shell, usuario no root.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/server /server

# Puerto por defecto del transporte Streamable HTTP (la plataforma puede
# inyectar otro vía PORT).
EXPOSE 8080

USER nonroot:nonroot
ENTRYPOINT ["/server"]
