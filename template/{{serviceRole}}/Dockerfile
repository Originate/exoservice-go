FROM golang
RUN curl https://glide.sh/get | sh
WORKDIR /go/src/app
COPY ./glide.lock .
COPY ./glide.yaml .
RUN glide install
COPY . .
