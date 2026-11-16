# Disposable "remote" host for e2e_test.go: MariaDB plus sshd and rsync.
# Root logs in with a key installed by the test; MariaDB root uses an empty
# password so plain `mysqldump -uroot` and `mysql -uroot` work over ssh.
FROM mariadb:lts
RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends openssh-server rsync \
    && rm -rf /var/lib/apt/lists/* \
    && ln -sf /usr/bin/mariadb /usr/local/bin/mysql \
    && ln -sf /usr/bin/mariadb-dump /usr/local/bin/mysqldump \
    && mkdir -p /run/sshd /root/.ssh \
    && chmod 700 /root/.ssh \
    && ssh-keygen -A \
    && printf 'PermitRootLogin prohibit-password\nPasswordAuthentication no\n' > /etc/ssh/sshd_config.d/e2e.conf
ENV MARIADB_ALLOW_EMPTY_ROOT_PASSWORD=1
CMD ["sh", "-c", "/usr/sbin/sshd && exec docker-entrypoint.sh mariadbd"]
