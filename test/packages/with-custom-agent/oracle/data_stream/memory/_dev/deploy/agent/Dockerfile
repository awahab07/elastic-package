ARG ES_AGENT_IMAGE=docker.elastic.co/elastic-agent/elastic-agent-complete:latest
FROM $ES_AGENT_IMAGE
USER root
RUN if grep wolfi /etc/os-release > /dev/null ; then \
      apk update && apk add libaio wget unzip; \
    else \
      apt-get update && apt-get -y install libaio1 wget unzip; \
    fi
WORKDIR /opt/oracle
RUN wget https://download.oracle.com/otn_software/linux/instantclient/214000/instantclient-basic-linux.x64-21.4.0.0.0dbru.zip && unzip -o instantclient-basic-linux.x64-21.4.0.0.0dbru.zip
RUN wget https://download.oracle.com/otn_software/linux/instantclient/217000/instantclient-sqlplus-linux.x64-21.7.0.0.0dbru.zip && unzip -o instantclient-sqlplus-linux.x64-21.7.0.0.0dbru.zip
RUN mkdir -p /etc/ld.so.conf.d && echo /opt/oracle/instantclient_21_4 > /etc/ld.so.conf.d/oracle-instantclient.conf && ldconfig
RUN cp /opt/oracle/instantclient_21_7/glogin.sql /opt/oracle/instantclient_21_7/libsqlplus.so /opt/oracle/instantclient_21_7/libsqlplusic.so /opt/oracle/instantclient_21_7/sqlplus /opt/oracle/instantclient_21_4/
RUN export LD_LIBRARY_PATH=/opt/oracle/instantclient_21_4:$LD_LIBRARY_PATH  && export PATH=/opt/oracle/instantclient_21_7:$PATH
ENV LD_LIBRARY_PATH="${LD_LIBRARY_PATH}:/opt/oracle/instantclient_21_4"
ENV PATH="${PATH}:/opt/oracle/instantclient_21_4"
WORKDIR /opt/oracle/instantclient_21_4
