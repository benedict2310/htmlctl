FROM golang:1.24-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/htmlctl ./cmd/htmlctl && \
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/htmlservd ./cmd/htmlservd

FROM caddy:2.8.4 AS caddy-bin

FROM debian:bookworm-slim AS htmlservd
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata && rm -rf /var/lib/apt/lists/*
RUN useradd --system --uid 10001 --create-home --home-dir /home/htmlservd --shell /bin/bash htmlservd
COPY --from=build /out/htmlservd /usr/local/bin/htmlservd
COPY --from=caddy-bin /usr/bin/caddy /usr/local/bin/caddy
RUN apt-get update && apt-get install -y --no-install-recommends libcap2-bin && \
	setcap cap_net_bind_service=+ep /usr/local/bin/caddy && \
	apt-get purge -y --auto-remove libcap2-bin && \
	rm -rf /var/lib/apt/lists/*
COPY docker/htmlservd-entrypoint.sh /usr/local/bin/htmlservd-entrypoint.sh
RUN chmod 755 /usr/local/bin/htmlservd-entrypoint.sh
RUN mkdir -p /var/lib/htmlservd /etc/caddy && chown -R 10001:10001 /var/lib/htmlservd /etc/caddy
ENV HTMLSERVD_BIND=0.0.0.0 \
	HTMLSERVD_PORT=9400 \
	HTMLSERVD_DATA_DIR=/var/lib/htmlservd \
	HTMLSERVD_CADDY_BINARY=/usr/local/bin/caddy \
	HTMLSERVD_CADDYFILE_PATH=/etc/caddy/Caddyfile
EXPOSE 80 443 9400
USER 10001:10001
ENTRYPOINT ["/usr/local/bin/htmlservd-entrypoint.sh"]

FROM htmlservd AS htmlservd-ssh
USER root
RUN apt-get update && apt-get install -y --no-install-recommends openssh-server && rm -rf /var/lib/apt/lists/*
RUN mkdir -p /run/sshd /etc/ssh/sshd_config.d /root/.ssh && \
	printf '%s\n' \
		'Port 22' \
		'PasswordAuthentication no' \
		'KbdInteractiveAuthentication no' \
		'PermitRootLogin no' \
		'PubkeyAuthentication yes' \
		'AuthorizedKeysFile .ssh/authorized_keys' \
		'AllowUsers htmlservd' \
		'AllowTcpForwarding yes' \
		'PermitTunnel yes' \
		'GatewayPorts no' \
		'X11Forwarding no' > /etc/ssh/sshd_config.d/htmlservd.conf
COPY docker/htmlservd-ssh-entrypoint.sh /usr/local/bin/htmlservd-ssh-entrypoint.sh
RUN chmod 755 /usr/local/bin/htmlservd-ssh-entrypoint.sh
EXPOSE 22 9400
ENTRYPOINT ["/usr/local/bin/htmlservd-ssh-entrypoint.sh"]

FROM debian:bookworm-slim AS htmlctl
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates openssh-client && rm -rf /var/lib/apt/lists/*
RUN useradd --system --uid 10001 --create-home --home-dir /home/htmlctl --shell /usr/sbin/nologin htmlctl
COPY --from=build /out/htmlctl /usr/local/bin/htmlctl
RUN mkdir -p /home/htmlctl/.ssh && chown -R 10001:10001 /home/htmlctl
USER 10001:10001
ENTRYPOINT ["/usr/local/bin/htmlctl"]
