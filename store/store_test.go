package store

import (
	"github.com/stretchr/testify/assert"
	"testing"
	//"github.com/stretchr/testify/suite"
	"fmt"
)

func TestStore(t *testing.T) {
	store := New()
	//store.Set("/clusters", true, nil)
	values := make(map[string]string)
	for i := 1; i <= 10; i++ {
		values[fmt.Sprintf("/clusters/%v/ip", i)] = fmt.Sprintf("192.168.0.%v", i)
		values[fmt.Sprintf("/clusters/%v/name", i)] = fmt.Sprintf("cluster-%v", i)
	}
	store.SetBulk(values)

	val, ok := store.Get("/clusters/10")
	assert.True(t, ok)

	fmt.Printf("%v", val)

	val, ok = store.Get("/clusters/1/ip")
	assert.True(t, ok)
	assert.Equal(t, "192.168.0.1", val)
}
