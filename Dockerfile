FROM golang:latest as build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

COPY .env .env 

RUN CGO_ENABLED=0 GOOS=linux go build -a  -o main ./main.go

FROM alpine:latest

WORKDIR /app


COPY --from=build /app/main /app/
COPY --from=build /app/.env /app/


# Add a non-root user
RUN adduser -D appuser
RUN chown -R appuser:appuser /app
USER appuser

CMD ["./main"]