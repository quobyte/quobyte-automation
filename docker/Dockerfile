FROM java:8-jre

ENV QUOBYTE_REPO_ID <QUOBYTE_REPO_ID>

RUN apt-get update && \
    apt-get install -y \
    apt-transport-https \
    python-software-properties

RUN wget -q https://support.quobyte.com/repo/2/${QUOBYTE_REPO_ID}/xUbuntu_14.10/Release.key -O - | apt-key add - && \
    echo "deb https://support.quobyte.com/repo/2/${QUOBYTE_REPO_ID}/xUbuntu_14.10 ./" > /etc/apt/sources.list.d/quobyte.list

RUN apt-get update && \
    apt-get install -y \
    quobyte-server \
    quobyte-client \
    dnsutils

COPY ./main.sh /opt/main.sh
ENTRYPOINT ["/opt/main.sh"]
