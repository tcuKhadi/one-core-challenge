FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . ./
RUN go mod tidy
RUN CGO_ENABLED=0 go build -trimpath -o /server .

FROM alpine:3.21
WORKDIR /app
COPY --from=build /server /app/server
EXPOSE 8080
# Server is PID 1 so kill -9 spot checks test the database recovery directly
CMD ["/app/server"]
