FROM openjdk:7-jre

RUN curl https://spark-nomad.s3.amazonaws.com/spark-2.1.1-bin-nomad.tgz | tar -xzC /tmp
RUN mv /tmp/spark* /opt/spark

ENV SPARK_HOME /opt/spark
ENV PATH $PATH:$SPARK_HOME/bin
