# syntax=docker/dockerfile:1

# --- build stage ---------------------------------------------------------
FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/coordinator ./cmd/coordinator
RUN CGO_ENABLED=0 go build -trimpath -o /out/trainer ./cmd/trainer

# --- runtime stage -------------------------------------------------------
FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/coordinator /app/coordinator
COPY --from=build /out/trainer /app/trainer

CMD ["/app/coordinator"]
