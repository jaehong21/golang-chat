package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-redis/redis/v7"
	"github.com/gorilla/mux"
	"github.com/jaehong21/golang-chat/api"
)

var rdb *redis.Client

func main() {
	rdb = redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer rdb.Close()
	// rdb.SAdd(user.ChannelsKey, "general", "random")

	r := mux.NewRouter()

	r.Path("/").Methods("GET").HandlerFunc(api.HealthCheck)
	r.Path("/chat").Methods("GET").HandlerFunc(api.H(rdb, api.ChatWebSocketHandler))
	r.Path("/users").Methods("GET").HandlerFunc(api.H(rdb, api.UsersHandler))
	r.Path("/user/{user}/channels").Methods("GET").HandlerFunc(api.H(rdb, api.UserChannelsHandler))

	fmt.Println("Successfully connected to redis")
	fmt.Println("Server is running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
