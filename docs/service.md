# Run metad as service

## docker

```
$ docker run -d -p 8080:80 -p 9611:9611 --restart=always qingcloud/metad
```

## systemd

create metad service:

```shell
cat <<EOF > /etc/systemd/system/metad.service
[Unit]
Description=metad

[Service]
ExecStart=/usr/local/bin/metad
Restart=always

[Install]
WantedBy=multi-user.target
EOF
```

start metad service:

```
$ systemctl enable metad.service
$ systemctl start  metad.service
```

## systemd & docker

create metad service:

```shell
cat <<EOF > /etc/systemd/system/metad.service
[Unit]
Description=metad
After=docker.service
Requires=docker.service

[Service]
TimeoutStartSec=0
ExecStartPre=-/usr/bin/docker kill metad-in-docker
ExecStartPre=-/usr/bin/docker rm metad-in-docker
ExecStartPre=/usr/bin/docker pull qingcloud/metad
ExecStart=/usr/bin/docker run --name metad-in-docker qingcloud/metad
Restart=always

[Install]
WantedBy=multi-user.target
EOF
```

start metad service:

```
$ systemctl enable metad.service
$ systemctl start  metad.service
```
