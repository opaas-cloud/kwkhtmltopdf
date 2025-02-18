# first stage: build kwkhtmltopdf_server

ARG TARGETARCH

FROM docker.io/golang:1.23.3
WORKDIR /tmp
COPY server/kwkhtmltopdf_server.go .
RUN go build kwkhtmltopdf_server.go

# second stage: server with wkhtmltopdf

FROM docker.io/ubuntu:22.04

RUN set -x \
  && apt update \
  && apt -y install --no-install-recommends \
    wget \
    ca-certificates \
    fonts-liberation2 \
    fontconfig \
    libjpeg-turbo8 \
    libx11-6 \
    libxext6 \
    libxrender1 \
    xfonts-75dpi \
    xfonts-base \
    fonts-lato \
  && wget -q -O /tmp/wkhtmltox.deb https://github.com/wkhtmltopdf/packaging/releases/download/0.12.6.1-2/wkhtmltox_0.12.6.1-2.jammy_${TARGETARCH}.deb \
  && dpkg -i /tmp/wkhtmltox.deb \
  && apt -f install \
  && apt -y purge wget --autoremove \
  && apt -y clean \
  && rm -rf /var/lib/apt/lists/* \
  && rm /tmp/wkhtmltox.deb

COPY --from=0 /tmp/kwkhtmltopdf_server /usr/local/bin/

RUN adduser --disabled-password --gecos '' kwkhtmltopdf
USER kwkhtmltopdf
ENV LANG=C.UTF-8
ENV LC_ALL=C.UTF-8

EXPOSE 8080
CMD ["/usr/local/bin/kwkhtmltopdf_server"]
