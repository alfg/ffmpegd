###############################
# Build the ffmpegd-build image.
FROM golang:1.26-alpine AS build

WORKDIR /go/src/ffmpegd
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go install -v .

##########################
# Build the release image.
FROM alfg/ffmpeg:latest
LABEL maintainer="Alfred Gutierrez <alf.g.jr@gmail.com>"

WORKDIR /home
ENV PATH=/opt/bin:$PATH
# Listen on all interfaces inside the container, or the published port can't
# reach it. Limit exposure with the port mapping instead.
ENV FFMPEGD_HOST=0.0.0.0

COPY --from=build /go/bin/ffmpegd /opt/bin/ffmpegd

EXPOSE 8080

CMD ["ffmpegd"]