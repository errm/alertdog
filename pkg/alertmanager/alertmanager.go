package alertmanager

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/prometheus/alertmanager/client"
	"github.com/prometheus/client_golang/api"
	"go.uber.org/atomic"
)

type Alertmanager struct {
	Endpoints []string
	Expiry    time.Duration
}

func (a Alertmanager) Alert(alert Alert) error {
	clientAlert := alert.clientAlert()
	now := time.Now()
	clientAlert.StartsAt = now
	clientAlert.EndsAt = now.Add(a.Expiry)
	return a.push(clientAlert)
}

func (a Alertmanager) Resolve(alert Alert) error {
	clientAlert := alert.clientAlert()
	now := time.Now()
	clientAlert.StartsAt = now
	clientAlert.EndsAt = now
	return a.push(clientAlert)
}

// push sends the alerts to all configured Alertmanagers concurrently
// It returns an error if the alerts could not be sent successfully to at least one Alertmanager.
// Somewhat based upon https://github.com/prometheus/prometheus/blob/main/notifier/notifier.go
func (a Alertmanager) push(alert client.Alert) error {
	var (
		pushes atomic.Int64
		wg     sync.WaitGroup
	)

	for _, endpoint := range a.Endpoints {
		wg.Add(1)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		go func(address string) {
			defer wg.Done()
			apiClient, err := api.NewClient(api.Config{Address: address})
			if err != nil {
				log.Printf("Error configuring apiclient for %s - %s", address, err)
				return
			}
			alertClient := client.NewAlertAPI(apiClient)
			err = alertClient.Push(ctx, alert)
			if err != nil {
				log.Printf("Error pushing alert to %s - %s", address, err)
				return
			}
			pushes.Inc()
		}(endpoint)
	}

	wg.Wait()

	if pushes.Load() < 1 {
		return errors.New("Failed to push alert to any alertmanager")
	}

	return nil
}
