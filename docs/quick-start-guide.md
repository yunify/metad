# Quick Start Guide

Before we begin be sure to [download and install metad](installation.md).

## Select a backend

metad supports the following backends:

* local  memory store,just for test
* etcd v3 api

The following example we use etcd v3 as a backend

## Start etcd

``` bash
etcd
```

## Start metad

We enable --xff just for fake request ip.

```
metad --backend etcdv3 --nodes http://127.0.0.1:2379 --log_level debug --listen :8080 --xff
```

## Put data

We can direct use backend api to put data, also can put data by metad's manage api.

### Put data by etcdctl

``` bash
export ETCDCTL_API=3

for i in `seq 1 5`; 
do  
    etcdctl put /nodes/$i/name node$i; 
    etcdctl put /nodes/$i/ip 192.168.1.$i;
done
```

### Put metadata by metad

Metad data manage api support put a whole json object.

```
curl -X PUT -H "Content-Type: application/json" http://127.0.0.1:9611/v1/data -d '
{
	"nodes": {
		"1": {
			"ip": "192.168.1.1",
			"name": "node1"
		},
		"2": {
			"ip": "192.168.1.2",
			"name": "node2"
		},
		"3": {
			"ip": "192.168.1.3",
			"name": "node3"
		},
		"4": {
			"ip": "192.168.1.4",
			"name": "node4"
		},
		"5": {
			"ip": "192.168.1.5",
			"name": "node5"
		}
	}
}'
```

## Update and delete metadata

If we use PUT method to update a node with a json object, will add or update the dir's sub node. If we use POST method, will replace the whole node's content with new value.

```
curl -X PUT -H "Content-Type: application/json" http://127.0.0.1:9611/v1/data/nodes -d '{"6":{"ip":"192.168.1.6","name":"node6"}}'
```

We show the metadata by GET data manage's api

```
curl -H "Accept: application/json" http://127.0.0.1:9611/v1/data

{
	"nodes": {
		"1": {
			"ip": "192.168.1.1",
			"name": "node1"
		},
		"2": {
			"ip": "192.168.1.2",
			"name": "node2"
		},
		"3": {
			"ip": "192.168.1.3",
			"name": "node3"
		},
		"4": {
			"ip": "192.168.1.4",
			"name": "node4"
		},
		"5": {
			"ip": "192.168.1.5",
			"name": "node5"
		},
		"6": {
			"ip": "192.168.1.6",
			"name": "node6"
		}
	}
}
```

We add a label object to /nodes/6

```
curl -X PUT -H "Content-Type: application/json" http://127.0.0.1:9611/v1/data/nodes/6 -d '{"label":{"key1":"value1"}}'

curl -H "Accept: application/json" -H "X-Forwarded-For: 192.168.1.1" http://127.0.0.1:9611/v1/data/nodes/6

{"ip":"192.168.1.6","label":{"key1":"value1"},"name":"node6"}
```

We add a new label object to /nodes/6

```
curl -X PUT -H "Content-Type: application/json" http://127.0.0.1:9611/v1/data/nodes/6 -d '{"label":{"key2":"value2"}}'

curl -H "Accept: application/json" -H "X-Forwarded-For: 192.168.1.1" http://127.0.0.1:9611/v1/data/nodes/6

{"ip":"192.168.1.6","label":{"key1":"value1","key2":"value2"},"name":"node6"}
```

We update a label key2 by PUT method

```
curl -X PUT -H "Content-Type: application/json" http://127.0.0.1:9611/v1/data/nodes/6 -d '{"label":{"key2":"new_value2"}}'

curl -H "Accept: application/json" -H "X-Forwarded-For: 192.168.1.1" http://127.0.0.1:9611/v1/data/nodes/6

{"ip":"192.168.1.6","label":{"key1":"value1","key2":"new_value2"},"name":"node6"}
```

We replace whole label object by POST method

```
curl -X POST -H "Content-Type: application/json" http://127.0.0.1:9611/v1/data/nodes/6/label -d '{"key3":"value3"}'

curl -H "Accept: application/json" -H "X-Forwarded-For: 192.168.1.1" http://127.0.0.1:9611/v1/data/nodes/6

{"ip":"192.168.1.6","label":{"key3":"value3"},"name":"node6"}
```

We can direct update leaf node value, request body must be a json value, so string must have quota

```
curl -X PUT -H "Content-Type: application/json" http://127.0.0.1:9611/v1/data/nodes/6/name -d '"new_node6"'
```

We can delete node's sub nodes, the follow command will delete /nodes/6/ip and /nodes/6/label, but /nodes/6/name will keep.

```
curl -X DELETE http://127.0.0.1:9611/v1/data/nodes/6?subs=ip,label
```

We delete whole /nodes/6 dir by command

```
curl -X DELETE http://127.0.0.1:9611/v1/data/6
```

## Show metadata 


We can show metadata by backend api, such as etcdctl

```
etcdctl get / --prefix

/nodes/1/ip
192.168.1.1
/nodes/1/name
node1
/nodes/2/ip
192.168.1.2
/nodes/2/name
node2
/nodes/3/ip
192.168.1.3
/nodes/3/name
node3
/nodes/4/ip
192.168.1.4
/nodes/4/name
node4
/nodes/5/ip
192.168.1.5
/nodes/5/name
node5

```

We also show metadata by metad's API. Metad's default output format is text. 

```
curl http://127.0.0.1:8080/

/nodes/1/ip      192.168.1.1
/nodes/1/name    nn_node1
/nodes/2/ip      192.168.1.2
/nodes/2/name    node2
/nodes/3/ip      192.168.1.3
/nodes/3/name    node3
/nodes/4/ip      192.168.1.4
/nodes/4/name    node4
/nodes/5/ip      192.168.1.5
/nodes/5/name    node5

```

We can add the request header "Accept: application/json", to let metad output json.

```
curl -H "Accept: application/json" http://127.0.0.1:8080/?pretty=true

{
    "nodes": {
        "1": {
            "ip": "192.168.1.1",
            "name": "node1"
        },
        "2": {
            "ip": "192.168.1.2",
            "name": "node2"
        },
        "3": {
            "ip": "192.168.1.3",
            "name": "node3"
        },
        "4": {
            "ip": "192.168.1.4",
            "name": "node4"
        },
        "5": {
            "ip": "192.168.1.5",
            "name": "node5"
        }
    }
}
```

Metad also support yaml output:

```
curl -H "Accept: application/yaml" http://127.0.0.1:8080/

nodes:
  "1":
    ip: 192.168.1.1
    name: node1
  "2":
    ip: 192.168.1.2
    name: node2
  "3":
    ip: 192.168.1.3
    name: node3
  "4":
    ip: 192.168.1.4
    name: node4
  "5":
    ip: 192.168.1.5
    name: node5
```

## Mapping and self request

Metad support self request by store a mapping of ip and metadata. 

We create mapping by metad's mapping manage api.

```
curl -H "Content-Type: application/json" -X PUT http://127.0.0.1:9611/v1/mapping -d '{"192.168.1.1":{"node":"/nodes/1"}, "192.168.1.2":{"node":"/nodes/2"}, "192.168.1.3":{"node":"/nodes/3"}}'
```
We can show mapping config by GET mapping manage api.

```
curl -H "Accept: application/json" http://127.0.0.1:9611/v1/mapping

{"192.168.1.1":{"node":"/nodes/1"},"192.168.1.2":{"node":"/nodes/2"},"192.168.1.3":{"node":"/nodes/3"}}
```

We use X-Forwarded-For to fake a request from 192.168.1.1, to demo self request.

```
curl -H "Accept: application/json" -H "X-Forwarded-For: 192.168.1.1" http://127.0.0.1:8080/self/node

{"ip":"192.168.1.1","name":"node1"}
```

We change ip to 192.168.1.2, will get node2 metadata.

```
curl -H "Accept: application/json" -H "X-Forwarded-For: 192.168.1.2" http://127.0.0.1:8080/self/node

{"ip":"192.168.1.2","name":"node2"}
```


### Waiting for a change

We can watch for a change on a key (include child keys) and receive a notification by using long polling.

```
curl -H "Accept: application/json" -H "X-Forwarded-For: 192.168.1.1" http://127.0.0.1:8080/self/?wait=true
```

Now we are waiting for any changes at path /self.

In another terminal, we update the node name of 192.168.1.1.

```
curl -X PUT -H "Content-Type: application/json" http://127.0.0.1:9611/v1/data/nodes/1/name -d '"n_node1"'
```

The first terminal should get the notification and return with latest result after change.

``` json
{"node":{"ip":"192.168.1.1","name":"nn_node1"}}
```

If we want to ensure that changes are not lost when the client lost connection to server, just add parameter **prev_version** to wait request.

```
curl -H "Accept: application/json" -H "X-Forwarded-For: 192.168.1.1" "http://127.0.0.1:8080/self/?wait=true&prev_version=10"
```

If metadata has been changed after version 10, this request will return immediately, else wait new change.
Everyone response's headers contains **X-Metad-Version** .
