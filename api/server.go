package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/danielpaulus/software-bluetooth-keyboard/hid"
	log "github.com/sirupsen/logrus"
)

func StartServer(keyboard hid.Keyboard) {
	// Create a mux for routing incoming requests
	m := http.NewServeMux()

	m.HandleFunc("/supportedKeys", func(w http.ResponseWriter, r *http.Request) {
		output, err := json.Marshal(hid.SupportedKeys())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("content-type", "application/json")
		w.Write(output)
	})

	// All URLs will be handled by this function
	m.HandleFunc("/sendKey", func(w http.ResponseWriter, r *http.Request) {
		b, err := ioutil.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		// Unmarshal
		key := string(b)
		if !hid.IsSupported(key) {
			http.Error(w, "specified key is not supported, call /supportedKeys for a list of supported keys", 400)
			return
		}

		if !keyboard.Status().IsReady {
			http.Error(w, "Not ready", 500)
			return
		}
		keyboard.TypeKey(key)
	})

	// All URLs will be handled by this function
	m.HandleFunc("/typeText", func(w http.ResponseWriter, r *http.Request) {
		b, err := ioutil.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		// Unmarshal
		text := string(b)
		if text == "" {
			http.Error(w, "empty text cannot be typed", 400)
			return
		}

		if !keyboard.Status().IsReady {
			http.Error(w, "Not ready", 500)
			return
		}
		keyboard.TypeText(text)
	})

	// Create a server listening on port 8000
	s := &http.Server{
		Addr:    ":8080",
		Handler: m,
	}
	log.Info("Starting REST API")
	// Continue to process new requests until an error occurs
	log.Fatal(s.ListenAndServe())
}
