# docker build -f dev/docker-dev-env/Dockerfile -t nomad-dev-env .
# docker run -ti -v /tmp:/tmp nomad-dev-env bash -c "cp /output/nomad /tmp/"
# adb root
# adb remount
# adb push /tmp/nomad /system/xbin/

# mkdir /data/data/nomad
# cd /data/data/nomad
# vim /data/system/packages.list
# nomad 10113 0 /data/user/0/nomad default:targetSdkVersion=27 3003
# nomad agent -config /storage/emulated/0/server.hcl
# ./nomad agent -config client1.hcl -network-interface=wlan0
# ./nomad agent -config client2.hcl -network-interface=wlan0

# ./dist/nomad job run dev/docker-dev-env/example.nomad

# nomad agent -dev -data-dir=`pwd` -network-interface=wlan0

FROM golang:rc

RUN apt update -y && \
    apt install -y \
        build-essential \
        git \
        liblxc1 \
        unzip \
	      wget \
        curl \
        python \
        apt-transport-https \
        libtinfo5 \
        clang

# --------------------- start android ndk --------------------------
WORKDIR /opt

ENV TOOLCHAIN_DIR /opt/android-ndk-linux

RUN wget -q --output-document=android-ndk.zip https://dl.google.com/android/repository/android-ndk-r19-linux-x86_64.zip && \
	  unzip android-ndk.zip && \
	  rm -f android-ndk.zip && \
	  mv android-ndk-r19 android-ndk-linux

# -------------- setup toolchain -----------------
WORKDIR /go/src/github.com/hashicorp/nomad
RUN mkdir /output

ENV CC $TOOLCHAIN_DIR/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android21-clang
ENV CXX $TOOLCHAIN_DIR/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android21-clang++
ENV GOOS android
ENV GOARCH arm64
ENV CGO_ENABLED 1

RUN go install std

RUN go install os/user

RUN go build -tags=nonvidia -o /output/nomad
