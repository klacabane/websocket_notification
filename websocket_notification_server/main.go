package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

var mutex = &sync.Mutex{}

type Notification struct {
	Receivers []int
	Channel   string
	Message   string
}

type Client struct {
	Id       int
	Conn     *websocket.Conn
	Channels []string
}

type UserMessage struct {
	Subscribe   []string
	Unsubscribe []string
}

var connections = make(map[int]*Client)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func main() {
	notify := make(chan Notification, 100)
	go func() {
		/**
		 * Broadcast the notifications to the connected clients.
		 */
		for {
			notification := <-notify

			logMessage := "Broadcasting notification "
			if len(notification.Receivers) > 0 {
				logMessage += fmt.Sprintf("for users (%v) ", notification.Receivers)
			}
			logMessage += fmt.Sprintf("on channel %s.", notification.Channel)

			log.Printf(logMessage)

			var success int
			var errors int
			for _, client := range connections {
				if len(notification.Receivers) > 0 && !intContains(notification.Receivers, client.Id) {
					continue
				}

				if strContains(client.Channels, notification.Channel) {
					err := client.Conn.WriteJSON(notification)
					if err != nil {
						errors++
					} else {
						success++
					}
				}
			}

			log.Printf("Notification successfully broadcasted to %d users with %d errors.", success, errors)
		}
	}()

	/**
	 * Receive notifications from the applications.
	 * This might be a hotspot so make sure we don't do heavy stuff and block resources.
	 * Maybe consider a queue for background processing like persisting the notification details in db.
	 */
	http.HandleFunc("/notification", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		var receivers []int
		for _, receiver := range query["receiver"] {
			id, _ := strconv.Atoi(receiver)
			receivers = append(receivers, id)
		}

		notify <- Notification{
			Channel:   query.Get("channel"),
			Message:   query.Get("message"),
			Receivers: receivers,
		}
	})

	/**
	 * Websocket endpoint.
	 * On connection, we might want to push the persisted notifications not delivered yet.
	 */
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		userId, _ := strconv.Atoi(r.Header.Get("X-User-Id"))

		mutex.Lock()
		client, ok := connections[userId]
		if !ok {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				log.Println(err)
				return
			}

			client = &Client{
				Id:       userId,
				Conn:     conn,
				Channels: strings.Split(r.Header.Get("X-Subscribe"), ","),
			}
			connections[userId] = client

			log.Printf("User id(%d) connected.", userId)
		}
		mutex.Unlock()

		for {
			// Listen for user subscribe/unsubscribe message.
			messageType, content, err := client.Conn.ReadMessage()
			if err != nil || messageType == websocket.CloseMessage {
				mutex.Lock()
				delete(connections, userId)
				mutex.Unlock()

				log.Printf("User id(%d) disconnected.", userId)
				return
			}

			var message UserMessage
			if err = json.Unmarshal(content, &message); err != nil {
				log.Printf("Error parsing message content to json. (%s)", string(content))
			}

			for _, channel := range message.Subscribe {
				client.Channels = append(client.Channels, channel)
				log.Printf("Subscribed user (%d) to channel %s.", userId, channel)
			}

			for _, channel := range message.Unsubscribe {
				ok, index := findIndex(client.Channels, channel)
				if ok {
					client.Channels = append(client.Channels[:index], client.Channels[index+1:]...)
					log.Printf("Unsubscribed user (%d) from channel %s.", userId, channel)
				}
			}
		}
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func intContains(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func strContains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func findIndex(s []string, e string) (found bool, index int) {
	var val string
	for index, val = range s {
		if val == e {
			found = true
			break
		}
	}
	return
}
