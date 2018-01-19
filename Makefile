# Copyright 2018 Yunify Inc. All rights reserved.
# Use of this source code is governed by a Apache license
# that can be found in the LICENSE file.

default:

graph:
	godepgraph \
		-o github.com/yunify/metad \
		-p github.com/yunify/metad/vendor \
		github.com/yunify/metad \
	| \
		dot -Tpng > import-graph.png

tools:
	go get github.com/kisielk/godepgraph

clean:
