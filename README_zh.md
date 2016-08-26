# metad


`metad` 是一个元数据服务,主要提供以下功能:

* **self** 语义支持,在服务器端对 IP 和元数据做映射,客户端直接通过 /self 请求和本节点相关的元数据.映射设置会保存到后端存储服务进行持久化.
* 元数据后端存储支持 [etcd](https://github.com/coreos/etcd) (TODO 支持更多后端).
* 元数据缓存,可以降低对后端(etcd)的请求压力.
* 输出格式支持json/yaml/text,对配置以及开发更友好.
* 支持作为 [confd](https://github.com/kelseyhightower/confd) 的后端服务(TODO).
