FROM golang:1.21-alpine

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o server main.go

EXPOSE 8080

CMD ["./server"]