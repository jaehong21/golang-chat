package user

import (
	"errors"
	"fmt"

	"github.com/go-redis/redis/v7"
)

const (
	usersKey       = "users"
	userChannelFmt = "user:%s:channels"
	ChannelsKey    = "channels"
)

type User struct {
	// name of the user
	name string
	// handler of the Redis subscribe command connection
	channelsHandler *redis.PubSub
	/*
		// cancel current subscription
		u.channelsHandler.Unsubscribe()
		u.channelsHandler.Close()

		// start a new subscription on the new channels list (old channels list + the new channel user ask to subscribe to)
		pubSub := rdb.Subscribe(channels...)
		u.channelsHandler = pubSub
	*/

	// used to stop current goroutines
	stopListenerChan chan struct{}
	listening        bool

	// golang channel between Redis subscription goroutine and the websocket goroutine
	MessageChan chan redis.Message
}

// Connect user to user channels on redis
func Connect(rdb *redis.Client, name string) (*User, error) {
	if _, err := rdb.SAdd(usersKey, name).Result(); err != nil {
		return nil, err
	}

	u := &User{
		name:             name,
		stopListenerChan: make(chan struct{}),
		MessageChan:      make(chan redis.Message),
	}

	if err := u.connect(rdb); err != nil {
		return nil, err
	}

	return u, nil
}

func (u *User) Subscribe(rdb *redis.Client, channel string) error {
	userChannelsKey := fmt.Sprintf(userChannelFmt, u.name)

	// If already subscribed to this channel, do nothing
	if rdb.SIsMember(userChannelsKey, channel).Val() {
		return nil
	}
	// Add subscribe channel
	if err := rdb.SAdd(userChannelsKey, channel).Err(); err != nil {
		return err
	}

	return u.connect(rdb)
}

func (u *User) Unsubscribe(rdb *redis.Client, channel string) error {
	userChannelsKey := fmt.Sprintf(userChannelFmt, u.name)

	if !rdb.SIsMember(userChannelsKey, channel).Val() {
		return nil
	}
	if err := rdb.SRem(userChannelsKey, channel).Err(); err != nil {
		return err
	}

	return u.connect(rdb)
}

// subscribes list of channels that user has
func (u *User) connect(rdb *redis.Client) error {
	var c []string

	c1, err := rdb.SMembers(ChannelsKey).Result()
	if err != nil {
		return err
	}
	c = append(c, c1...)

	// get all user channels (from DB) and start subscribe
	c2, err := rdb.SMembers(fmt.Sprintf(userChannelFmt, u.name)).Result()
	if err != nil {
		return err
	}
	c = append(c, c2...)

	if len(c) == 0 {
		fmt.Println("No channels to subscribe for user: ", u.name)
		return nil
	}

	// clean up previous subscription and connection
	if u.channelsHandler != nil {
		if err := u.channelsHandler.Unsubscribe(); err != nil {
			return err
		}
		if err := u.channelsHandler.Close(); err != nil {
			return err
		}
	}
	if u.listening {
		u.stopListenerChan <- struct{}{}
	}

	return u.doConnect(rdb, c...)
}

func (u *User) doConnect(rdb *redis.Client, channels ...string) error {
	// subscribe all channels in one request
	pubSub := rdb.Subscribe(channels...)

	// keep channel handler to be used in unsubscribe
	u.channelsHandler = pubSub

	// Listener Goroutine:
	// When getting a message from the Redis subscription, it will be sent to websocket connection
	go func() {
		u.listening = true
		fmt.Println("starting the listener for user:", u.name, "on channels:", channels)
		for {
			select {
			case msg, ok := <-pubSub.Channel():
				if !ok {
					return
				}
				u.MessageChan <- *msg
			case <-u.stopListenerChan:
				fmt.Println("stopping the listener for user:", u.name)
				break
			}
		}
	}()

	return nil
}

func (u *User) Disconnect() error {
	if u.channelsHandler != nil {
		if err := u.channelsHandler.Unsubscribe(); err != nil {
			return err
		}
		if err := u.channelsHandler.Close(); err != nil {
			return err
		}
	}
	if u.listening {
		u.stopListenerChan <- struct{}{}
	}
	close(u.MessageChan)

	return nil
}

func Chat(rdb *redis.Client, channel string, content string) error {
	return rdb.Publish(channel, content).Err()
}

func List(rdb *redis.Client) ([]string, error) {
	return rdb.SMembers(usersKey).Result()
}

func GetChannels(rdb *redis.Client, username string) ([]string, error) {

	if !rdb.SIsMember(usersKey, username).Val() {
		return nil, errors.New("user not exists")
	}

	var c []string

	c1, err := rdb.SMembers(ChannelsKey).Result()
	if err != nil {
		return nil, err
	}
	c = append(c, c1...)

	// get all user channels (from DB) and start subscribe
	c2, err := rdb.SMembers(fmt.Sprintf(userChannelFmt, username)).Result()
	if err != nil {
		return nil, err
	}
	c = append(c, c2...)

	return c, nil
}
