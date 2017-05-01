package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	n := flag.Int("n", 1, "Number of clients")
	flag.Parse()

	rand.Seed(time.Now().Unix())

	// List of space-separated channels
	var channels string = strings.Join(flag.Args(), ",")
	var connections []*websocket.Conn
	for i := 0; i < *n; i++ {
		userId := rand.Intn(1000000) + 1

		dialer := &websocket.Dialer{}
		wsHeaders := http.Header{
			"Origin":                   {"http://localhost:8080"},
			"Sec-WebSocket-Extensions": {"permessage-deflate; client_max_window_bits, x-webkit-deflate-frame"},
			"X-User-Id":                {strconv.Itoa(userId)},
			"X-Subscribe":              {channels},
		}

		wsConn, _, err := dialer.Dial("ws://localhost:8080/ws", wsHeaders)
		if err != nil {
			log.Fatal(err)
			return
		}

		if *n == 1 {
			log.Printf("I'm a client (id %d) subscribed to the channels %s.\n\n", userId, channels)
		}

		connections = append(connections, wsConn)

		go func() {
			/**
			 * Listening for new notification.
			 */
			var notification Notification
			for {
				if *n == 1 {
					log.Println("Waiting for notification.")
				}

				if err := wsConn.ReadJSON(&notification); err != nil {
					return
				}

				if *n == 1 {
					log.Printf("Received notification for %s: %s.", notification.Channel, notification.Message)
					log.Println("---------------------------")
				}
			}
		}()

	}

	if *n > 1 {
		log.Printf("%d clients successfully connected.", len(connections))
	}

	go func() {
		for {
			var command string
			var channel string
			fmt.Scanf("%s %s", &command, &channel)

			if (command == "s" || command == "u") && len(channel) > 0 {
				message := UserMessage{}
				if command == "s" {
					log.Printf("Subscribing to channel %s.", channel)
					message.Subscribe = append(message.Subscribe, channel)
				} else if command == "u" {
					log.Printf("Unsubscribing to channel %s.", channel)
					message.Unsubscribe = append(message.Unsubscribe, channel)
				}

				for _, conn := range connections {
					if err := conn.WriteJSON(message); err != nil {
						return
					}
				}
			}
		}
	}()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	<-interrupt
	for _, conn := range connections {
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		conn.Close()
	}
}

type Notification struct {
	Channel string
	Message string
}

type UserMessage struct {
	Subscribe   []string
	Unsubscribe []string
}
