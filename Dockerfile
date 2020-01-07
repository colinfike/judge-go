# Build binary using golang image for use later
FROM golang:latest as builder

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o judge-go-bot .
# RUN CGO_ENABLED=0 GOOS=linux go build -a -o judge-go-bot .

# Define image for deploy
FROM alpine:latest  

RUN wget https://johnvansickle.com/ffmpeg/builds/ffmpeg-git-amd64-static.tar.xz && tar xvf ffmpeg-git-amd64-static.tar.xz && mv ffmpeg-git-*-amd64-static/ffmpeg /usr/local/bin/

WORKDIR /app

COPY --from=builder /app/judge-go-bot .

CMD ["./judge-go-bot"]
