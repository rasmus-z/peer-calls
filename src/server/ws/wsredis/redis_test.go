package wsredis_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/go-redis/redis/v7"
	"github.com/jeremija/peer-calls/src/server/ws"
	"github.com/jeremija/peer-calls/src/server/ws/wsmessage"
	"github.com/jeremija/peer-calls/src/server/ws/wsredis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"nhooyr.io/websocket"
)

const room = "myroom"

var serializer wsmessage.ByteSerializer

type MockWSWriter struct {
	out chan []byte
}

func NewMockWriter() *MockWSWriter {
	return &MockWSWriter{
		out: make(chan []byte),
	}
}

func (w *MockWSWriter) Write(ctx context.Context, typ websocket.MessageType, msg []byte) error {
	w.out <- msg
	return nil
}

func (w *MockWSWriter) Read(ctx context.Context) (typ websocket.MessageType, msg []byte, err error) {
	<-ctx.Done()
	err = ctx.Err()
	return
}

func serialize(t *testing.T, msg wsmessage.Message) []byte {
	data, err := serializer.Serialize(msg)
	require.Nil(t, err)
	return data
}

func configureRedis(t *testing.T) (*redis.Client, *redis.Client, func()) {
	subRedis := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	pubRedis := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	return pubRedis, subRedis, func() {
		pubRedis.Close()
		subRedis.Close()
	}
}

func getClientIDs(t *testing.T, a *wsredis.RedisAdapter) map[string]string {
	clientIDs, err := a.Clients()
	assert.Nil(t, err)
	return clientIDs
}

func TestRedisAdapter_add_remove_client(t *testing.T) {
	pub, sub, stop := configureRedis(t)
	defer stop()
	adapter1 := wsredis.NewRedisAdapter(pub, sub, "peercalls", room)
	adapter2 := wsredis.NewRedisAdapter(pub, sub, "peercalls", room)
	mockWriter1 := NewMockWriter()
	defer close(mockWriter1.out)
	client1 := ws.NewClient(mockWriter1)
	defer client1.Close()
	client1.SetMetadata("a")
	mockWriter2 := NewMockWriter()
	defer close(mockWriter2.out)
	client2 := ws.NewClient(mockWriter2)
	defer client2.Close()
	client2.SetMetadata("b")
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(2)

	for _, client := range []*ws.Client{client1, client2} {
		go func(client *ws.Client) {
			err := client.Subscribe(ctx, func(msg wsmessage.Message) {})
			assert.True(t, errors.Is(err, context.Canceled), "expected error to be context.Canceled, but was: %s", err)
			wg.Done()
		}(client)
	}

	assert.Nil(t, adapter1.Add(client1))
	t.Log("waiting for room join message broadcast (1)")
	assert.Equal(t, serialize(t, wsmessage.NewMessageRoomJoin(room, client1.ID(), "a")), <-mockWriter1.out)

	assert.Nil(t, adapter2.Add(client2))
	t.Log("waiting for room join message broadcast (2)")
	assert.Equal(t, serialize(t, wsmessage.NewMessageRoomJoin(room, client2.ID(), "b")), <-mockWriter1.out)
	assert.Equal(t, serialize(t, wsmessage.NewMessageRoomJoin(room, client2.ID(), "b")), <-mockWriter2.out)
	assert.Equal(t, map[string]string{client1.ID(): "a", client2.ID(): "b"}, getClientIDs(t, adapter1))
	assert.Equal(t, map[string]string{client1.ID(): "a", client2.ID(): "b"}, getClientIDs(t, adapter2))

	assert.Nil(t, adapter1.Remove(client1.ID()))
	t.Log("waiting for client id removal", client1.ID())
	leaveMessage, err := serializer.Deserialize(<-mockWriter2.out)
	assert.Nil(t, err)
	assert.Equal(t, wsmessage.NewMessageRoomLeave(room, client1.ID()), leaveMessage)
	assert.Equal(t, map[string]string{client2.ID(): "b"}, getClientIDs(t, adapter2))

	assert.Nil(t, adapter2.Remove(client2.ID()))
	assert.Equal(t, map[string]string{}, getClientIDs(t, adapter2))

	t.Log("stopping...")
	for _, stop := range []func() error{adapter1.Close, adapter2.Close} {
		err := stop()
		assert.Equal(t, nil, err)
	}
	cancel()
	wg.Wait()
}
