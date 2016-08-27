package main

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"testing"
)

func TestConfigFile(t *testing.T) {
	config := Config{
		Backend:      "etcd",
		LogLevel:     "debug",
		PIDFile:      "/var/run/metad.pid",
		EnableXff:    true,
		Prefix:       "/users/uid1",
		OnlySelf:     true,
		Listen:       ":8080",
		ListenManage: "127.0.0.1:8112",
		BasicAuth:    true,
		ClientCaKeys: "/opt/metad/client_ca_keys",
		ClientCert:   "/opt/metad/client_cert",
		ClientKey:    "/opt/metad/client_key",
		BackendNodes: []string{"192.168.11.1:2379", "192.168.11.2:2379"},
		Username:     "username",
		Password:     "password",
	}

	data, err := yaml.Marshal(config)
	assert.NoError(t, err)
	configFile, fileErr := ioutil.TempFile("/tmp", "metad")

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
