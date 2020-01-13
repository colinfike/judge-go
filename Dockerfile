# Build binary using golang image for use later
FROM golang:latest as builder

WORKDIR /app

COPY . .

RUN GOOS=linux go build -o judgego cmd/judgego/main.go

# Define image for deploy
FROM alpine:latest  

RUN apk add --no-cache \
        ca-certificates \
        libc6-compat

RUN wget https://johnvansickle.com/ffmpeg/builds/ffmpeg-git-amd64-static.tar.xz && tar xvf ffmpeg-git-amd64-static.tar.xz && mv ffmpeg-git-*-amd64-static/ffmpeg /usr/local/bin/

WORKDIR /app

COPY --from=builder /app/judgego .

CMD ["./judgego"]
