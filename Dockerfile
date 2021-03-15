###############################
# Build the ffmpegd-build image.
FROM golang:1.16-alpine as build

WORKDIR /go/src/ffmpegd
COPY . .

RUN go get -d -v ./...
RUN go install -v ./...

##########################
# Build the release image.
FROM alfg/ffmpeg:latest
LABEL MAINTAINER Alfred Gutierrez <alf.g.jr@gmail.com>

WORKDIR /home
ENV PATH=/opt/bin:$PATH

COPY --from=build /go/bin/ffmpegd /opt/bin/ffmpegd

EXPOSE 8080

CMD ["ffmpegd"]