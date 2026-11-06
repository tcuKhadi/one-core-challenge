FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go schema.sql ./
# go mod tidy resolves dependencies + fills go.sum (not committed from Windows)
RUN go mod tidy
RUN CGO_ENABLED=0 go build -trimpath -o /server .

FROM alpine:3.21
WORKDIR /app
COPY --from=build /server /app/server
EXPOSE 8080
# exec form: the server is PID 1, so the kill -9 spot check hits the app itself
CMD ["/app/server"]
