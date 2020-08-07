package client

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/prologic/msgbus"
)

func TestClientPublish(t *testing.T) {
	assert := assert.New(t)

	mb := msgbus.New(nil)

	server := httptest.NewServer(mb)
	defer server.Close()

	client := NewClient(server.URL, nil)

	err := client.Publish("hello", "hello world")
	assert.NoError(err)

	topic := mb.NewTopic("hello")
	expected := msgbus.Message{Topic: topic, Payload: []byte("hello world")}

	actual, ok := mb.Get(topic)
	assert.True(ok)
	assert.Equal(actual.ID, expected.ID)
	assert.Equal(actual.Topic, expected.Topic)
	assert.Equal(actual.Payload, expected.Payload)
}
