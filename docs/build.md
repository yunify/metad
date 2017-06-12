# Building

## Building from Source

Go 1.7+ is required to build metad, which uses the new vendor directory.

```
mkdir -p $GOPATH/src/github.com/yunify
git clone https://github.com/yunify/metad.git $GOPATH/src/github.com/yunify/metad
cd $GOPATH/src/github.com/yunify/metad
./build
```

You should now have metad in your `bin/` directory:

```
ls bin/

metad
```

Install to bin dir

```
sudo ./install
```


## Building from Source by docker for alpine

```
docker build . -t qingcloud/metad_builder_alpine -f Dockerfile.build.alpine
docker run -ti --rm -v $(pwd):/app qingcloud/metad_builder_alpine ./build
```

The above docker commands will produce binary in the local bin directory.

## Run by docker image

```
docker run -it qingcloud/metad
```