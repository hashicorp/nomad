FROM docker.mirror.hashicorp.services/node:12.19.0-alpine
RUN apk add --update --no-cache git make g++ automake autoconf libtool nasm libpng-dev

COPY ./package.json /website/package.json
COPY ./package-lock.json /website/package-lock.json
WORKDIR /website
RUN npm install
