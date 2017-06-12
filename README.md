metad
=====

English|[中文](README_zh.md)

[![Build Status](https://travis-ci.org/yunify/metad.svg?branch=master)](https://travis-ci.org/yunify/metad) [![Gitter](https://badges.gitter.im/yunify/metad.svg)](https://gitter.im/yunify/metad?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge)

`metad` is a metadata server support the following features:

* **self** semantic support. metad keep a mapping of IP and metadata, client direct request "/self", will get the metadata of current node. mapping settings is store to backend.
* metadata backend support [etcd](https://github.com/coreos/etcd) (TODO support more).
* support metadata local cache, so it can be used as a proxy to reducing the request pressure of backend (etcd).
* api out format support json/yaml/text,and is metadata/developer friendly data structure.
* support as [confd](https://github.com/yunify/confd)'s backend.
* support metadata access rule define.


## Installation

You can download the latest release from [GitHub](https://github.com/yunify/metad/releases)

* [build from source](docs/build.md)

## Getting Started

* [quick start guide](docs/quick-start-guide.md)

## Next steps

* [Metad configuration](docs/configuration.md)
* [Metad API document](docs/api.md)
* [Working with confd](docs/confd.md)


Check out the [docs directory](docs) for more docs.
