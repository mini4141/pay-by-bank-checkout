FROM golang:1.26-alpine AS build
ARG SERVICE
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY gen ./gen
COPY internal ./internal
COPY services ./services
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./services/${SERVICE}

FROM alpine:3.22
RUN adduser -D -u 10001 app
USER app
COPY --from=build /out/app /app/app
ENTRYPOINT ["/app/app"]
