FROM openjdk:7

ENV HADOOP_VERSION 2.7.3

RUN wget -O - http://apache.mirror.iphh.net/hadoop/common/hadoop-$HADOOP_VERSION/hadoop-$HADOOP_VERSION.tar.gz | tar xz -C /usr/local/
ENV HADOOP_PREFIX /usr/local/hadoop-$HADOOP_VERSION
ENV PATH $PATH:$HADOOP_PREFIX/bin

COPY core-site.xml $HADOOP_PREFIX/etc/hadoop/
