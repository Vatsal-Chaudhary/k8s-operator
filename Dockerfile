# syntax=docker/dockerfile:1

FROM golang:1.22 AS builder

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the operator binary once a main package exists at the repository root.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/kafka-lag-scaler .

FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /out/kafka-lag-scaler /kafka-lag-scaler

USER 65532:65532

ENTRYPOINT ["/kafka-lag-scaler"]
