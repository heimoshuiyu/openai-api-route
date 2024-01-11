FROM docker.io/golang:1.21 as builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN make

FROM alpine

COPY --from=builder /app/openai-api-route /openai-api-route

ENTRYPOINT ["/openai-api-route"]
