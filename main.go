package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"sync"

	"time"
)

var (
	flagURL = flag.String("tokens", "http://localhost/tokens/list", "tokens url")
	flagDir = flag.String("root", "/mnt/streams", "ts files root directory")

	mu     = &sync.RWMutex{}
	tokens = make(map[string]string)
)

func main() {
	log.Println("Starting...")

	flag.Parse()

	go updateTokens()

	fs := http.FileServer(http.Dir(*flagDir))
	http.Handle("/", mh(fs))

	http.ListenAndServe("127.0.0.1:8000", nil)
}

func mh(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") == "" {
			http.NotFound(w, r)
			return
		}
		mu.RLock()
		_, ok := tokens[r.URL.Query().Get("token")]
		mu.RUnlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func updateTokens() {
	for {
		time.Sleep(5 * time.Second)
		body := remote_get(*flagURL)
		if body == nil {
			continue
		}
		mu.Lock()
		err := json.Unmarshal(body, &tokens)
		mu.Unlock()
		if err != nil {
			log.Printf("updateTokens error: %v", err)
			continue
		}
	}
}

func remote_get(url string) []byte {
	response, err := http.Get(*flagURL)
	if err != nil {
		log.Printf("remote_get error: %v", err)
		return nil
	}
	defer response.Body.Close()
	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Printf("remote_get error: %v", err)
		return nil
	}

	return b
}
