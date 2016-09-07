# Metad API Document

Metad's api is http rest style, and divided to metadata api and management api.

* Metadata api, default listen address is ":80"
* Manage api, default listen address is "127.0.0.1:9661", only on localhost interface for security.

## API Response ContentType

API response content type default is text, client can add "Accept" header to request, for special content type.

* text text/plain
* json application/json
* yaml application/yaml,application/x-yaml,text/yaml,text/x-yaml"

## Metadata API

### GET /{nodePath}

This api for client get metadata, and will process self mapping by client ip.

return origin metadata, for json example.

```
{
	"nodes": {
		"1": {
			"ip": "192.168.1.1",
			"name": "nn_node1"
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
			"label": {
				"key3": "value3"
			},
			"name": "node6"
		}
	},
	"self": {
		"node": {
			"ip": "192.168.1.2",
			"name": "node2"
		}
	}
}
```

## Manage API

Manage API default port is 127.0.0.1:9611

### /v1/data[/{nodePath}] 

This api is for manage metadata

* GET show metadata.
* POST create or replace metadata. 
* PUT create or merge metadata.
* DELETE delete metadata.
    
### /v1/mapping[/{nodePath}] 

This api is for manage metadata's ip mapping

* GET show mapping config.
* POST create or replace mapping config. 
* PUT create or merge update mapping config.
* DELETE delete mapping config.

### POST /v1/resync

resync metadata and mapping from backend.
