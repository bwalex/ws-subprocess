package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
	"github.com/gorilla/websocket"
)

const controllerUrlDefault = "http://127.0.0.1/ws-controller"

var globalState = struct {
	controllerUrl	string
	httpClient	*http.Client
}{
	controllerUrl:	controllerUrlDefault,
	httpClient:	nil,
}

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Maximum message size allowed from peer.
	maxMessageSize = 8192

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Time to wait before force close on connection.
	closeGracePeriod = 10 * time.Second
)

type CommandResponse struct {
	Command	string		`json:"command"`
	Args	[]string	`json:"args"`
}

func validateAndGetCommand(token string) (*CommandResponse, error) {
 	uri := globalState.controllerUrl

 	req, err := http.NewRequest("GET", uri, nil)
 	if err != nil {
 		log.Printf("ERR: http request setup failed: %v", err)
 		return nil, err
 	}

	q := req.URL.Query()
	q.Add("token", token)
	req.URL.RawQuery = q.Encode()

 	resp, err := globalState.httpClient.Do(req)
 	if err != nil {
 		log.Printf("ERR: http request failed: %v", err)
 		return nil, err
 	}
 	defer resp.Body.Close()

 	if resp.StatusCode != http.StatusOK {
 		log.Printf("ERR: http request returned status %s", resp.Status)
 		return nil, fmt.Errorf("HTTP request error: %s", resp.Status)
 	}

	var cmdResp CommandResponse
	err = json.NewDecoder(resp.Body).Decode(&cmdResp)
	if err != nil {
		log.Printf("ERR: json decoding failed: %v", err)
		return nil, err
	}

	return &cmdResp, nil
}

func pumpStdin(ws *websocket.Conn, w io.Writer) {
	defer ws.Close()
	ws.SetReadLimit(maxMessageSize)
	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		_, reader, err := ws.NextReader()
		if err != nil {
			return
		}
		io.Copy(w, reader)
	}
}

func pumpStdout(ws *websocket.Conn, r io.Reader, done chan struct{}) {
	ws.SetWriteDeadline(time.Time{})

	for {
		buf := make([]byte, 1024)
		read, err := r.Read(buf)
		if err != nil {
			break
		}
		err = ws.WriteMessage(websocket.BinaryMessage, buf[:read])
		if err != nil {
			ws.Close()
			break
		}
	}

	close(done)

	ws.SetWriteDeadline(time.Now().Add(writeWait))
	ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	time.Sleep(closeGracePeriod)
	ws.Close()
}

func ping(ws *websocket.Conn, done chan struct{}) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := ws.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeWait)); err != nil {
				log.Println("ping:", err)
			}
		case <-done:
			return
		}
	}
}

func internalError(ws *websocket.Conn, msg string, err error) {
	log.Println(msg, err)
	ws.WriteMessage(websocket.TextMessage, []byte("Internal server error."))
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func serveWs(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Token missing", 400)
		return
	}

	cmdResp, err := validateAndGetCommand(token)
	if err != nil {
		http.Error(w, "Invalid token", 403)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Upgrade failed", 400)
		return
	}

	defer ws.Close()

	outr, outw, err := os.Pipe()
	if err != nil {
		internalError(ws, "stdout:", err)
		return
	}
	defer outr.Close()
	defer outw.Close()

	inr, inw, err := os.Pipe()
	if err != nil {
		internalError(ws, "stdin:", err)
		return
	}
	defer inr.Close()
	defer inw.Close()

	log.Printf("Executing command: %s with args: %v", cmdResp.Command, cmdResp.Args)
	proc, err := os.StartProcess(cmdResp.Command, append([]string{cmdResp.Command}, cmdResp.Args...), &os.ProcAttr{
		Files: []*os.File{inr, outw, outw},
	})
	if err != nil {
		internalError(ws, "start:", err)
		return
	}

	inr.Close()
	outw.Close()

	stdoutDone := make(chan struct{})
	go pumpStdout(ws, outr, stdoutDone)
	go ping(ws, stdoutDone)

	pumpStdin(ws, inw)

	// Some commands will exit when stdin is closed.
	inw.Close()

	// Other commands need a bonk on the head.
	if err := proc.Signal(os.Interrupt); err != nil {
		log.Println("inter:", err)
	}

	select {
	case <-stdoutDone:
	case <-time.After(time.Second):
		// A bigger bonk on the head.
		if err := proc.Signal(os.Kill); err != nil {
			log.Println("term:", err)
		}
		<-stdoutDone
	}

	if _, err := proc.Wait(); err != nil {
		log.Println("wait:", err)
	}
}

func main() {
	controllerUrlPtr := flag.String("controller-url", controllerUrlDefault, "URL of controller")
	listenAddrPtr := flag.String("listen", "127.0.0.1:8866", "Listen address")

	flag.Parse()

	globalState.controllerUrl = *controllerUrlPtr
	globalState.httpClient = &http.Client{}

	http.HandleFunc("/ws", serveWs)
	log.Fatal(http.ListenAndServe(*listenAddrPtr, nil))
}
