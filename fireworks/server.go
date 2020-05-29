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
	_ "time"

	"github.com/gorilla/websocket"
)

var localtest = false

type ImageChooser struct {
	defaultImage string
}

func NewImageChooser() ImageChooser {
	return ImageChooser{defaultImage: "images/heart.jpeg"}
}

// Return the filename of an image, and a text string.
// This is pretty stupid, but works well enough for demo code.
func (x *ImageChooser) chooseImage(instr string) string {
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
		return x.defaultImage
	}
}

// Read an image and turn it into a base64 encoded string.
// Stackoverflow says don't use base64, but they weren't super clear on what to
// use instead, so we are where we are.
func (ImageChooser) smooshImage(filename string) (string, error) {
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

// Firework becomes a json payload sent to the client via the websocket
type Firework struct {
	Words string
	Image string
}

// IncomingRequest is what the client sends to the server.
type IncomingRequest struct {
	Color  string
	Person string
}

type OpenSocket struct {
	ws *websocket.Conn // Websocket connection
}

// write words and an image to the websocket.
func (x *OpenSocket) writeToSocket(message Firework) error {
	if err := x.ws.WriteJSON(message); err != nil {
		return err
	}
	return nil
}

func (x *OpenSocket) sendMessages(buffer []Firework) error {
	for _, message := range buffer {
		err := x.writeToSocket(message)
		if err != nil {
			log.Println(err)
		}
	}
	return nil
}

// Listen for requests from this socket and channel them back to the server to
// react to them.
func (x *OpenSocket) listen(channel chan<- IncomingRequest) {
	for {
		var fromClient IncomingRequest
		_, received, err := x.ws.ReadMessage()
		json.Unmarshal([]byte(received), &fromClient)
		log.Printf("Received message: %+v", fromClient)
		if err != nil {
			log.Println("Couldn't read message: ", err)
		}

		channel <- fromClient
	}
}

// FireworksServer handles sending fireworks and receiving requests for them.
type FireworksServer struct {
	// TODO: remove dead sockets
	sockets []*OpenSocket

	// TODO: make this a ring buffer
	// TODO: also cache repeated images
	buffer []Firework

	imageChooser ImageChooser

	requestChannel chan IncomingRequest
}

func NewFireworksServer() *FireworksServer {
	return &FireworksServer{
		// TODO: better slice sizes
		sockets:        []*OpenSocket{},
		buffer:         []Firework{},
		imageChooser:   ImageChooser{},
		requestChannel: make(chan IncomingRequest),
	}
}

// listen for firework requests and create fireworks
func (x *FireworksServer) makeFireworks() {
	fmt.Println("Ready to create fireworks")
	for {
		req := <-x.requestChannel
		fmt.Println("Got a message on the channel:", req)

		// TODO: validate the message

		m := Firework{}

		m.Words = fmt.Sprintf("%s: %s", req.Person, req.Color)
		imageFile := x.imageChooser.chooseImage(strings.ToLower(req.Color))

		fmt.Println("Chose image:", imageFile)
		if imageFile != "" {
			encodedImage, err := x.imageChooser.smooshImage(imageFile)
			if err != nil {
				log.Printf("Couldn't encode image: %v", err)
				continue
			}
			m.Image = encodedImage
			x.buffer = append(x.buffer, m)
		}

		for _, socket := range x.sockets {
			err := socket.writeToSocket(m)
			if err != nil {
				log.Println("Couldn't write to socket: ", err)
			}
		}
	}
}

// websocket endpoint. Streams firework pictures.
func (x *FireworksServer) wsEndpoint(w http.ResponseWriter, r *http.Request) {
	// TODO: are these good buffer numbers?
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	// Ignore CORS when testing locally.
	// TODO: pass localtest in as a variable
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
	log.Printf("Websocket connection established with %s", x.GetIP(r))

	socket := &OpenSocket{ws}
	x.sockets = append(x.sockets, socket)

	err = socket.sendMessages(x.buffer)
	if err != nil {
		log.Println(err)
	}

	err = socket.writeToSocket(Firework{Words: "Welcome to the websockets demo fireworks display. Every piece of software spontaneously generates its own chat functionality. Nobody knows why. This is no exception", Image: ""})

	if err != nil {
		log.Println(err)
	}
	socket.listen(x.requestChannel)
}

// GetIP gets a requests IP address by reading off the forwarded-for
// header (for proxies) and falls back to use the remote address.
func (FireworksServer) GetIP(r *http.Request) string {
	forwarded := r.Header.Get("X-FORWARDED-FOR")
	if forwarded != "" {
		return forwarded
	}
	return r.RemoteAddr
}

// Serve index.html
func (FireworksServer) homeEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	http.ServeFile(w, r, "index.html")
}

func main() {
	localtest = true // Flip this when testing locally.
	port := 80
	if localtest {
		log.SetOutput(os.Stdout)
		port = 8080
	} else {
		logFile, err := os.OpenFile("logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			log.Fatal(err)
		}
		log.SetOutput(logFile)
	}

	log.Println("Welcome to some terrible websockets test code.")

	server := NewFireworksServer()
	go server.makeFireworks()

	http.HandleFunc("/fireworks", server.homeEndpoint) // regular
	http.HandleFunc("/ws", server.wsEndpoint)          // upgraded to websocket
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
