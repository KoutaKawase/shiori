FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y \
        ca-certificates \
        curl \
        git \
        build-essential \
        pkg-config \
        sqlite3 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace/shiori

CMD ["/bin/bash"]
