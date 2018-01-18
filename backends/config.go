// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package backends

type Config struct {
	Backend      string
	Prefix       string
	Group        string
	BasicAuth    bool
	ClientCaKeys string
	ClientCert   string
	ClientKey    string
	BackendNodes []string
	Password     string
	Username     string
}
