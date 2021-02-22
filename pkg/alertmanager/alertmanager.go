package alertmanager

import (
	"context"
	"log"
	"time"

	"github.com/prometheus/alertmanager/client"
	"github.com/prometheus/client_golang/api"
)

type Alertmanager struct {
	Endpoints []string
	Expiry    time.Duration
}

func (a Alertmanager) Alert(alert Alert) error {
	clientAlert := alert.clientAlert()
	clientAlert.StartsAt = time.Now()
	clientAlert.EndsAt = time.Now().Add(a.Expiry)
	return a.push(clientAlert)
}

func (a Alertmanager) Resolve(alert Alert) error {
	clientAlert := alert.clientAlert()
	clientAlert.StartsAt = time.Now()
	clientAlert.EndsAt = time.Now()
	return a.push(clientAlert)
}

func (a Alertmanager) push(alert client.Alert) error {
	var err error
	var pushes int
	for _, endpoint := range a.Endpoints {
		apiClient, err := api.NewClient(api.Config{Address: endpoint})
		if err != nil {
			log.Println(err)
			continue
		}
		alertClient := client.NewAlertAPI(apiClient)
		err = alertClient.Push(
			context.Background(),
			alert,
		)
		if err != nil {
			log.Println(err)
			continue
		}
		pushes += 1
	}
	if pushes < 1 {
		return err
	}
	return nil
}
