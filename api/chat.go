package api

import (
	"fmt"
	"github.com/go-redis/redis/v7"
	"github.com/gorilla/websocket"
	"github.com/jaehong21/golang-chat/user"
	"net/http"
)

var upgrader websocket.Upgrader
var connectedUser = make(map[string]*user.User)

func H(rdb *redis.Client, fn func(http.ResponseWriter, *http.Request, *redis.Client)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, rdb)
	}
}

type msg struct {
	Content string `json:"content,omitempty"`
	Channel string `json:"channel,omitempty"`
	Command int    `json:"command,omitempty"`
	Err     string `json:"err,omitempty"`
}

const (
	commandSubscribe = iota
	commandUnsubscribe
	commandChat
)

func ChatWebSocketHandler(w http.ResponseWriter, r *http.Request, rdb *redis.Client) {
	// upgrade http to websocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		handleWsError(err, conn)
		return
	}

	err = onConnect(r, conn, rdb)
	if err != nil {
		handleWsError(err, conn)
		return
	}

	// get channel to disconnect
	closeCh := onDisconnect(r, conn, rdb)

	// on receive message from redis channel
	onChannelMessage(conn, r)

loop:
	for {
		select {
		case <-closeCh:
			break loop
		default:
			onUserMessage(conn, r, rdb)
		}
	}
}

func onConnect(r *http.Request, conn *websocket.Conn, rdb *redis.Client) error {
	username := r.URL.Query()["username"][0]
	fmt.Println("connected from:", conn.RemoteAddr(), "user:", username)

	u, err := user.Connect(rdb, username)
	if err != nil {
		return err
	}
	connectedUser[username] = u
	return nil
}

func onDisconnect(r *http.Request, conn *websocket.Conn, rdb *redis.Client) chan struct{} {
	closeCh := make(chan struct{})
	username := r.URL.Query()["username"][0]

	conn.SetCloseHandler(func(code int, text string) error {
		fmt.Println("disconnected user:", username)
		u := connectedUser[username]
		if err := u.Disconnect(); err != nil {
			return err
		}
		delete(connectedUser, username)
		close(closeCh)
		return nil
	})

	return closeCh
}

func onUserMessage(conn *websocket.Conn, r *http.Request, rdb *redis.Client) {
	var msg msg
	if err := conn.ReadJSON(&msg); err != nil {
		handleWsError(err, conn)
		return
	}

	username := r.URL.Query()["username"][0]
	u := connectedUser[username]

	switch msg.Command {
	case commandSubscribe:
		if err := u.Subscribe(rdb, msg.Channel); err != nil {
			handleWsError(err, conn)
		}
	case commandUnsubscribe:
		if err := u.Unsubscribe(rdb, msg.Channel); err != nil {
			handleWsError(err, conn)
		}
	case commandChat:
		if err := user.Chat(rdb, msg.Channel, msg.Content); err != nil {
			handleWsError(err, conn)
		}
	}
}

func onChannelMessage(conn *websocket.Conn, r *http.Request) {
	username := r.URL.Query()["username"][0]
	u := connectedUser[username]

	go func() {
		for m := range u.MessageChan {
			msg := msg{
				Content: m.Payload,
				Channel: m.Channel,
			}
			if err := conn.WriteJSON(msg); err != nil {
				fmt.Println(err)
			}
		}
	}()

}

func handleWsError(err error, conn *websocket.Conn) {
	_ = conn.WriteJSON(msg{Err: err.Error()})
}
