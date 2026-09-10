package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/errm/alertdog/pkg/alertdog"
)

func main() {
	a := readConfig()
	a.Setup()
	go a.CheckLoop()
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	http.Handle("/webhook", a)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", a.Port), nil))
}

func readConfig() *alertdog.Alertdog {
	var a *alertdog.Alertdog
	configFile, err := os.ReadFile("config.yml")
	if err != nil {
		log.Fatal(err)
	}
	err = yaml.Unmarshal(configFile, &a)
	if err != nil {
		log.Fatal(err)
	}
	return a
}
