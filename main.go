package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"

	"time"
)

var (
	flagURL  = flag.String("tokens", "http://localhost/tokens/list", "tokens url")
	flagDir  = flag.String("root", "/mnt/streams", "ts files root directory")
	flagBind = flag.String("bind", "127.0.0.1:8000", "bind on host:port")
	flagV2   = flag.Bool("v2", false, "-v2")
	flagFE   = flag.String("fe", "127.0.0.1", "-fe 127.0.0.1")

	mu     = &sync.RWMutex{}
	tokens = make(map[string]string)
)

type T struct {
	ADDR map[string]bool   `json:"addr"`
	IP   map[string]string `json:"ip"`
	CH   map[string]string `json:"ch"`
}

var (
	tokens2 = &T{make(map[string]bool), make(map[string]string), make(map[string]string)}
)

func main() {
	log.Println("Starting...")

	flag.Parse()

	go updateTokens()

	fs := http.FileServer(http.Dir(*flagDir))
	mh := mh_v1
	if *flagV2 {
		mh = mh_v2
	}
	http.Handle("/", mh(fs))

	http.HandleFunc("/reload", func(w http.ResponseWriter, r *http.Request) {
		if *flagV2 {
			tokens_v2()
		} else {
			tokens_v1()
		}
	})

	http.ListenAndServe(*flagBind, nil)
}

func mh_v1(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isTs := strings.HasSuffix(r.URL.String(), ".ts")
		if r.URL.Query().Get("token") == "" && !isTs {
			http.NotFound(w, r)
			return
		}
		mu.RLock()
		_, ok := tokens[r.URL.Query().Get("token")]
		mu.RUnlock()
		if !ok && !isTs {
			http.NotFound(w, r)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func mh_v2(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remote_ip := strings.Split(r.RemoteAddr, ":")[0]
		isTs := strings.HasSuffix(r.URL.String(), ".ts")
		if isTs && remote_ip == *flagFE {
			mu.RLock()
			_, ok := tokens2.ADDR[r.Header.Get("X-Forwarded-For")]
			mu.RUnlock()
			if !ok {
				http.NotFound(w, r)
				return
			}
		}
		if r.URL.Query().Get("token") == "" && !isTs {
			http.NotFound(w, r)
			return
		}
		mu.RLock()
		ip, ok := tokens2.IP[r.URL.Query().Get("token")]
		mu.RUnlock()
		if !ok && !isTs {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("X-Forwarded-For") != ip && remote_ip == *flagFE && !isTs {
			http.NotFound(w, r)
			return
		}
		parts := strings.Split(r.URL.String(), "/")
		if len(parts) < 2 && !isTs {
			http.NotFound(w, r)
			return
		}
		mu.RLock()
		ch, ok := tokens2.CH[r.URL.Query().Get("token")]
		mu.RUnlock()
		if !ok && !isTs {
			http.NotFound(w, r)
			return
		}
		if ch != parts[1] && !isTs {
			http.NotFound(w, r)
			return
		}

		h.ServeHTTP(w, r)
	})
}

func updateTokens() {
	if *flagV2 {
		for {
			tokens_v2()
			time.Sleep(5 * time.Second)
		}
	} else {
		for {
			tokens_v1()
			time.Sleep(5 * time.Second)
		}
	}
}

func tokens_v1() {
	body := remote_get(*flagURL)
	if body == nil {
		return
	}
	mu.Lock()
	err := json.Unmarshal(body, &tokens)
	mu.Unlock()
	if err != nil {
		log.Printf("updateTokens error: %v", err)
		return
	}
}

func tokens_v2() {
	body := remote_get(*flagURL)
	if body == nil {
		return
	}
	mu.Lock()
	err := json.Unmarshal(body, &tokens2)
	mu.Unlock()
	if err != nil {
		log.Printf("updateTokens error: %v", err)
		return
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
