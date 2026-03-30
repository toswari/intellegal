FROM golang:1.26.1-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/go-api ./cmd/server

FROM golang:1.26.1-alpine AS dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && go install github.com/air-verse/air@v1.63.0

COPY . .

EXPOSE 8080

CMD ["air", "-c", ".air.toml"]

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/go-api /app/go-api

EXPOSE 8080

ENTRYPOINT ["/app/go-api"]
