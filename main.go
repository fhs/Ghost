/*
This program is a GhostText plugin for Acme text editor.

GhostText (https://github.com/GhostText/GhostTex) is a browser extension
that allows you to edit text in web pages (e.g. in <textarea>) in various
text editors. It interacts with the text editor via a websocket server,
such as this program.

By default, this program will listen on localhost port 4001 (GhostText default
port). This can be changed with the -port flag. When this program receives
a new edit request from GhostText, it'll open a new acme window. Middle
clicking on "Put" sends text from acme to the web page. After editing
is done, when GhostText closes the websocket connection, the acme window
will be automatically closed if it's not dirty.
*/
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"9fans.net/go/acme"
	"github.com/gorilla/websocket"
)

var port = flag.Int("port", 4001, "websocket listen port")

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Selection struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type WebText struct {
	Selections []Selection `json:"selections"`
	Syntax     string      `json:"syntax"`
	Title      string      `json:"title"`
	Text       string      `json:"text"`
	URL        string      `json:"url"`
}

func handleWinEvents(w *acme.Win, conn *websocket.Conn) {
	for e := range w.EventChan() {
		switch e.C2 {
		case 'x', 'X': // execute
			if string(e.Text) == "Put" {
				body, err := w.ReadAll("body")
				if err != nil {
					log.Printf("failed to read body: %v\n", err)
				}
				w.Ctl("addr=dot")
				q0, q1, _ := w.ReadAddr()
				web := &WebText{
					Selections: []Selection{{q0, q1}},
					Syntax:     "text.plain",
					Text:       string(body),
				}
				if err := conn.WriteJSON(web); err != nil {
					log.Printf("WriteJSON failed: %v\n", err)
				}
				w.Ctl("clean")
				continue
			}
		}
		w.WriteEvent(e)
	}
	w.CloseFiles()
}

func handler(wr http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Upgrade") != "websocket" {
		wr.Header().Set("Content-Type", "application/json")
		greeting := fmt.Sprintf(`{"WebSocketPort":%v,"ProtocolVersion":1}`, *port)
		fmt.Fprintf(wr, "%v\n", greeting)
		return
	}
	conn, err := upgrader.Upgrade(wr, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v\n", err)
		return
	}
	w, err := acme.New()
	if err != nil {
		log.Printf("acme.New failed: %v\n", err)
		return
	}
	w.Write("tag", []byte("Undo Redo Put"))
	go handleWinEvents(w, conn)
	for {
		var web WebText
		if err := conn.ReadJSON(&web); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNoStatusReceived) {
				log.Printf("error: %v, user-agent: %v", err, r.Header.Get("User-Agent"))
			}
			w.Del(false)
			return
		}
		w.Name(fmt.Sprintf("/Ghost/%v", web.URL))
		w.Addr(",")
		w.Write("data", []byte(web.Text))
		w.Ctl("clean")

		if len(web.Selections) > 0 {
			sel := web.Selections[0]
			w.Addr("#%v,#%v", sel.Start, sel.End)
			w.Ctl("dot=addr")
		}
	}
}

func main() {
	flag.Parse()
	http.HandleFunc("/", handler)
	if err := http.ListenAndServe(fmt.Sprintf("localhost:%v", *port), nil); err != nil {
		log.Fatalf("ListenAndServe failed: %v", err)
	}
}
