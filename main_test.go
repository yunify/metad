package main

import (
	"fmt"
	"github.com/docker/distribution/Godeps/_workspace/src/gopkg.in/yaml.v2"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
)

func TestConfigFile(t *testing.T) {
	config := Config{
		Backend:      "etcd",
		LogLevel:     "debug",
		PIDFile:      "/var/run/metadata-proxy.pid",
		EnableXff:    true,
		Prefix:       "/users/uid1",
		OnlySelf:     true,
		Listen:       ":8080",
		ListenManage: "127.0.0.1:8112",
		BasicAuth:    true,
		ClientCaKeys: "/opt/metadata-proxy/client_ca_keys",
		ClientCert:   "/opt/metadata-proxy/client_cert",
		ClientKey:    "/opt/metadata-proxy/client_key",
		BackendNodes: []string{"192.168.11.1:2379", "192.168.11.2:2379"},
		Username:     "username",
		Password:     "password",
	}

	data, err := yaml.Marshal(config)
	assert.NoError(t, err)
	configFile, fileErr := ioutil.TempFile("/tmp", "metadata-proxy")

	fmt.Printf("configFile: %v \n", configFile.Name())

	assert.Nil(t, fileErr)
	c, ioErr := configFile.Write(data)
	assert.Nil(t, ioErr)
	assert.Equal(t, len(data), c)

	config2 := Config{}
	loadErr := loadConfigFile(configFile.Name(), &config2)
	assert.Nil(t, loadErr)

	assert.Equal(t, config, config2)
}
