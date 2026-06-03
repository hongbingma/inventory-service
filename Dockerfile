FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/inventory-service ./cmd/inventory-service

FROM alpine:3.22
RUN adduser -D -H appuser
USER appuser
COPY --from=build /out/inventory-service /inventory-service
EXPOSE 8000 9000
ENTRYPOINT ["/inventory-service"]
