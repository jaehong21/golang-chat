package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-redis/redis/v7"
	"github.com/gorilla/mux"
	"github.com/jaehong21/golang-chat/user"
)

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func UserChannelsHandler(w http.ResponseWriter, r *http.Request, rdb *redis.Client) {
	username := mux.Vars(r)["user"]

	list, err := user.GetChannels(rdb, username)
	if err != nil {
		handleError(err, w)
		return
	}
	err = json.NewEncoder(w).Encode(list)
	if err != nil {
		handleError(err, w)
		return
	}

}

func UsersHandler(w http.ResponseWriter, r *http.Request, rdb *redis.Client) {
	list, err := user.List(rdb)
	if err != nil {
		handleError(err, w)
		return
	}
	err = json.NewEncoder(w).Encode(list)
	if err != nil {
		handleError(err, w)
		return
	}
}

func handleError(err error, w http.ResponseWriter) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(fmt.Sprintf(`{"err": "%s"}`, err.Error())))
}
