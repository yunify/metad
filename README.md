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
bin/metadata-proxy --backend etcdv3 --nodes 127.0.0.1:2379 --log_level debug --listen :8080 --xff true
```

* set etcd version

```
export ETCDCTL_API=3
```

* fill data to etcd

```
for i in `seq 1 5`; 
do  
    etcdctl put /nodes/$i/name node$i; 
    etcdctl put /nodes/$i/ip 192.168.11.$i;
done
```

* show data

by etcdctl

```
etcdctl get / --prefix

/nodes/1/ip
192.168.11.1
/nodes/1/name
node1
/nodes/2/ip
192.168.11.2
/nodes/2/name
node2
/nodes/3/ip
192.168.11.3
/nodes/3/name
node3
/nodes/4/ip
192.168.11.4
/nodes/4/name
node4
/nodes/5/ip
192.168.11.5
/nodes/5/name
node5

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
