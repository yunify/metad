# Installation

### Binary Download

Currently metad ships binaries for OS X and Linux 64bit systems. You can download the latest release from [GitHub](https://github.com/yunify/metad/releases)

#### OS X

```
$ wget https://github.com/yunify/metad/releases/download/v1.0-alpha.4/metad-darwin-amd64
```

#### Linux

```
$ wget https://github.com/yunify/metad/releases/download/v1.0-alpha.4/metad-linux-amd64
```

#### Building from Source

Go 1.6+ is required to build metad, which uses the new vendor directory.

```
$ mkdir -p $GOPATH/src/github.com/yunify
$ git clone https://github.com/yunify/metad.git $GOPATH/src/github.com/yunify/metad
$ cd $GOPATH/src/github.com/yunify/metad
$ ./build
```

You should now have metad in your `bin/` directory:

```
$ ls bin/
metad
```

Install to bin dir

```
$ sudo ./install
```


#### Building from Source by docker

```
$ docker build -t metad_builder -f Dockerfile.build .
$ docker run -ti --rm -v $(pwd):/app metad_builder ./build
```


The above docker commands will produce binary in the local bin directory.

#### Building release docker image

```
$ docker build -t metad .
```

### Next Steps

Get up and running with the [Quick Start Guide](quick-start-guide.md).
