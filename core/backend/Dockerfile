#importing golang version on linux then named builder
FROM golang:1.26-alpine AS builder

#anchoring directory
WORKDIR /app

#copy go.mod and go.sum inside /app (. means in current directory anchor)
COPY go.mod go.sum ./

# go inside go.mod and download all libraries and package in the internet
RUN go mod download

# . - copy all inside my folder, . - put the copy inside the current directory anchor
COPY . .

# RUN - execute in terminal inside docker, CGO_ENABLED=0 - disables C code integration
# GOOS=linux - compile to linux, go build - command to compiles code
# -o main - output, named it main, . - go look at direction /app
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# download alpine linux image latest
FROM alpine:latest

# go to /root/ directory
WORKDIR /root/

# go back to builder then grab the main then put in the current directory which is /root/
COPY --from=builder /app/main .

# port of backend
EXPOSE 2026

# copy .env
COPY .env .

# copy the db migrations
COPY db/ ./db/

# run this command by default when its run on realworld
CMD ["./main"]