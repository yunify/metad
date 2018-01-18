// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package local

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yunify/metad/log"
	"github.com/yunify/metad/store"
)

func init() {
	log.SetLevel("debug")
	rand.Seed(int64(time.Now().Nanosecond()))
}

func TestClientSyncStop(t *testing.T) {

	stopChan := make(chan bool)

	storeClient, err := NewLocalClient()
	assert.NoError(t, err)

	go func() {
		time.Sleep(3000 * time.Millisecond)
		stopChan <- true
	}()

	metastore := store.New()
	// expect internalSync not block after stopChan has signal
	storeClient.internalSync("data", storeClient.data, metastore, stopChan)
}
