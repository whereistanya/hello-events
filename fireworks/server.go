package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// The json payload sent via the websocket
type msg struct {
	Words  string
	Image  string
	Person string
}

var localtest = false

// TODO: make this a ring buffer
// TODO: also cache repeated images
// TODO: also not a global variable wtf
var buffer = []msg{}

// TODO: same, global variable
// TODO: remove dead sockets
var sockets = []*websocket.Conn{}

var defaultImage = "images/heart.jpeg"

// Return the filename of an image, and a text string.
// This is pretty stupid, but works well enough for demo code.
// I use she-ra images when running this locally to entertain my kid,
// but swapping in placeholders on github because obv I don't have
// rights to the She-ra images.
func chooseImage(instr string) string {
	heart := "images/heart.jpeg"
	colors := map[string]string{
		"blue":   "images/blue.jpeg",
		"green":  "images/green.jpeg",
		"orange": "images/orange.jpeg",
		"pink":   "images/pink.jpeg",
		"purple": "images/purple.jpeg",
		"red":    "images/red.jpeg",
		"white":  "images/white.jpeg",
		"yellow": "images/yellow.jpeg",
	}

	found, ok := colors[instr]
	if ok {
		return found
	} else {
		return heart
	}
}

// Serve index.html
func homeEndpoint(w http.ResponseWriter, r *http.Request) {
	log.Printf("HTTP connection from %s", GetIP(r))
	w.Header().Set("Content-Type", "text/html")
	http.ServeFile(w, r, "index.html")
}

func backfillFireworks(ws *websocket.Conn) error {
	for _, message := range buffer {
		err := writeToSocket(ws, message)
		if err != nil {
			log.Println(err)
		}
	}
	return nil
}

// websocket endpoint. Streams firework pictures.
func wsEndpoint(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	// Ignore CORS when testing locally.
	if localtest {
		upgrader.CheckOrigin = func(r *http.Request) bool {
			return true
		}
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Couldn't upgrade connection: %v", err)
		return
	}
	log.Printf("Websocket connection established with %s", GetIP(r))
	sockets = append(sockets, ws)

	err = backfillFireworks(ws)

	err = writeToSocket(ws, msg{Words: "Welcome to the websockets demo fireworks display. Every piece of software spontaneously generates its own chat functionality. Nobody knows why.", Image: ""})

	if err != nil {
		log.Printf("Couldn't write to client: %v", err)
	}

	waitTime := time.Duration(60)

	type request struct {
		Color  string
		Person string
	}
	c1 := make(chan request, 1)
	for {
		// Spinning this off into a goroutine so we can put a timer on it and take
		// some indecisive action if there's no request for a while.
		go func() {
			var fromClient request
			_, received, err := ws.ReadMessage()
			json.Unmarshal([]byte(received), &fromClient)
			log.Printf("Received message: %+v", fromClient)
			if err != nil {
				log.Println("Couldn't read message: ", err)
			} else {
				c1 <- fromClient // string(fromClient.Color)
			}
		}()

		m := msg{}

		select {
		case rec := <-c1: // got something from the client
			req := rec
			m.Words = fmt.Sprintf("%s: %s", req.Person, req.Color)
			imageFile := chooseImage(strings.ToLower(req.Color))

			if imageFile != "" {
				encodedImage, err := smooshImage(imageFile)
				if err != nil {
					log.Printf("Couldn't encode image: %v", err)
					continue
				}
				m.Image = encodedImage
				buffer = append(buffer, m)
			}

			for _, socket := range sockets {
				err = writeToSocket(socket, m)
				if err != nil {
					log.Println("Couldn't write to socket: ", err)
					continue
				}
			}

		case <-time.After(waitTime * time.Second): // nothing for a while; send a prompt
			waitTime = waitTime + 1 // longer timeout next time
			imageFile := defaultImage
			encodedImage, err := smooshImage(imageFile)
			if err != nil {
				log.Printf("Couldn't encode image: %v", err)
				continue
			}
			m.Image = encodedImage
			m.Words = "Choose a firework <3 <3"
			// Send only to this one socket.
			err = writeToSocket(ws, m)
		}

	}
}

// GetIP gets a requests IP address by reading off the forwarded-for
// header (for proxies) and falls back to use the remote address.
func GetIP(r *http.Request) string {
	forwarded := r.Header.Get("X-FORWARDED-FOR")
	if forwarded != "" {
		return forwarded
	}
	return r.RemoteAddr
}

// Read an image and turn it into a base64 encoded string.
// Stackoverflow says don't use base64, but they weren't super clear on what to
// use instead, so we are where we are.
func smooshImage(filename string) (string, error) {
	imageFile, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer imageFile.Close()

	loadedImage, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}

	encoded := base64.StdEncoding.EncodeToString(loadedImage)
	ext := filepath.Ext(filename)

	if ext == "png" {
		encoded = "data:image/png;base64, " + encoded
	} else if ext == "jpg" || ext == "jpeg" {
		encoded = "data:image/jpeg;base64, " + encoded
	}
	return encoded, nil
}

// write words and an image to the websocket.
func writeToSocket(conn *websocket.Conn, message msg) error {
	if err := conn.WriteJSON(message); err != nil {
		return err
	}
	return nil
}

func main() {
	localtest = true // Flip this when testing locally.
	port := 80
	logFile, err := os.OpenFile("logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}

	log.SetOutput(logFile)
	if localtest {
		log.SetOutput(os.Stdout)
		port = 8080
	}
	log.Println("Welcome to some terrible websockets test code.")
	http.HandleFunc("/fireworks", homeEndpoint) // regular
	http.HandleFunc("/ws", wsEndpoint)          // upgraded to websocket
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
