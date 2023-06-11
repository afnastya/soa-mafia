FROM golang:1.18

WORKDIR /app

RUN mkdir -p build
COPY . .
RUN go mod download

RUN go build -o /app/build/mafia_server ./server/main.go

CMD ["./build/mafia_server"]