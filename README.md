# metadata-proxy


`metadata-proxy` is a metadata server support the following features:

* **self** semantic support. metadata-proxy keep a mapping of IP and metadata, client direct request "/self", will get the metadata of this node. mapping settings is store to backend (etcd).
* metadata backend support [etcd](https://github.com/coreos/etcd) (TODO support more).
* support metadata local cache, so it can be used as a proxy to reducing the request pressure of backend (etcd).
* api out format support json/yaml/text,and is metadata/developer friendly data structure.
* support as [confd](https://github.com/kelseyhightower/confd) backend(TODO).

[中文文档](README_zh.md)

## Building

Go 1.6 is required to build confd, which uses the new vendor directory.

```
$ mkdir -p $GOPATH/src/github.com/yunify
$ git clone https://github.com/yunify/metadata-proxy.git $GOPATH/src/github.com/yunify/metadata-proxy
$ cd $GOPATH/src/github.com/yunify/metadata-proxy
$ ./build
```

You should now have metadata-proxy in your `bin/` directory:

```
$ ls bin/
metadata-proxy
```

## Getting Started

* start etcd

```
etcd
```

* start metadata-proxy

```
bin/metadata-proxy --backend etcd --nodes 127.0.0.1:2379 --log_level debug --listen :8080 --xff true
```

* fill data to etcd

```
for i in `seq 1 5`; 
do  
    etcdctl set /nodes/$i/name node$i; 
    etcdctl set /nodes/$i/ip 192.168.11.$i;
done
```

* show data

by etcdctl

```
etcdctl ls -r /

/nodes
/nodes/1
/nodes/1/name
/nodes/1/ip
/nodes/2
/nodes/2/name
/nodes/2/ip
/nodes/3
/nodes/3/name
/nodes/3/ip
/nodes/4
/nodes/4/ip
/nodes/4/name
/nodes/5
/nodes/5/name
/nodes/5/ip
```

by etcd api

```
curl "http://127.0.0.1:2379/v2/keys/?recursive=true"

{
    "action": "get",
    "node": {
        "dir": true,
        "nodes": [
            {
                "key": "/nodes",
                "dir": true,
                "nodes": [
                    {
                        "key": "/nodes/4",
                        "dir": true,
                        "nodes": [
                            {
                                "key": "/nodes/4/ip",
                                "value": "192.168.11.4",
                                "modifiedIndex": 6955,
                                "createdIndex": 6955
                            },
                            {
                                "key": "/nodes/4/name",
                                "value": "node4",
                                "modifiedIndex": 6954,
                                "createdIndex": 6954
                            }
                        ],
                        "modifiedIndex": 6954,
                        "createdIndex": 6954
                    },
                    {
                        "key": "/nodes/5",
                        "dir": true,
                        "nodes": [
                            {
                                "key": "/nodes/5/name",
                                "value": "node5",
                                "modifiedIndex": 6956,
                                "createdIndex": 6956
                            },
                            {
                                "key": "/nodes/5/ip",
                                "value": "192.168.11.5",
                                "modifiedIndex": 6957,
                                "createdIndex": 6957
                            }
                        ],
                        "modifiedIndex": 6956,
                        "createdIndex": 6956
                    },
                    {
                        "key": "/nodes/1",
                        "dir": true,
                        "nodes": [
                            {
                                "key": "/nodes/1/name",
                                "value": "node1",
                                "modifiedIndex": 6948,
                                "createdIndex": 6948
                            },
                            {
                                "key": "/nodes/1/ip",
                                "value": "192.168.11.1",
                                "modifiedIndex": 6949,
                                "createdIndex": 6949
                            }
                        ],
                        "modifiedIndex": 6948,
                        "createdIndex": 6948
                    },
                    {
                        "key": "/nodes/2",
                        "dir": true,
                        "nodes": [
                            {
                                "key": "/nodes/2/name",
                                "value": "node2",
                                "modifiedIndex": 6950,
                                "createdIndex": 6950
                            },
                            {
                                "key": "/nodes/2/ip",
                                "value": "192.168.11.2",
                                "modifiedIndex": 6951,
                                "createdIndex": 6951
                            }
                        ],
                        "modifiedIndex": 6950,
                        "createdIndex": 6950
                    },
                    {
                        "key": "/nodes/3",
                        "dir": true,
                        "nodes": [
                            {
                                "key": "/nodes/3/name",
                                "value": "node3",
                                "modifiedIndex": 6952,
                                "createdIndex": 6952
                            },
                            {
                                "key": "/nodes/3/ip",
                                "value": "192.168.11.3",
                                "modifiedIndex": 6953,
                                "createdIndex": 6953
                            }
                        ],
                        "modifiedIndex": 6952,
                        "createdIndex": 6952
                    }
                ],
                "modifiedIndex": 6948,
                "createdIndex": 6948
            }
        ]
    }
}
```

by metadata-proxy text output

```
curl http://127.0.0.1:8080/

nodes/
```

by metadata-proxy json output

```
curl -H "Accept: application/json" http://127.0.0.1:8080/

{
    "nodes": {
        "1": {
            "ip": "192.168.11.1",
            "name": "node1"
        },
        "2": {
            "ip": "192.168.11.2",
            "name": "node2"
        },
        "3": {
            "ip": "192.168.11.3",
            "name": "node3"
        },
        "4": {
            "ip": "192.168.11.4",
            "name": "node4"
        },
        "5": {
            "ip": "192.168.11.5",
            "name": "node5"
        }
    }
}
```

by metadata-proxy yaml output

```
curl -H "Accept: application/yaml" http://127.0.0.1:8080/

nodes:
  "1":
    ip: 192.168.11.1
    name: node1
  "2":
    ip: 192.168.11.2
    name: node2
  "3":
    ip: 192.168.11.3
    name: node3
  "4":
    ip: 192.168.11.4
    name: node4
  "5":
    ip: 192.168.11.5
    name: node5
```

register self mapping

```
curl http://127.0.0.1:8112/v1/register -d 'ip=192.168.11.1&mapping={"node":"/nodes/1"}'

OK
```

self request

```
curl -H "X-Forwarded-For: 192.168.11.1" http://127.0.0.1:8080/

nodes/
self/

curl -H "X-Forwarded-For: 192.168.11.1" http://127.0.0.1:8080/self/node

ip
name

curl -H "X-Forwarded-For: 192.168.11.1" http://127.0.0.1:8080/self/node/ip

192.168.11.1

```
