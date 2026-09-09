package pagerduty

import (
	"log"
	"sync"

	"github.com/PagerDuty/go-pagerduty"
)

type Client interface {
	ManageEvent(event pagerduty.V2Event) (*pagerduty.V2EventResponse, error)
}

type Notifier struct {
	routingKey string
	runbookURL string
	client     Client

	mu        sync.Mutex
	triggered map[string]bool
}

func New(routingKey, runbookURL string, client Client) *Notifier {
	if client == nil {
		client = clientFunc(pagerduty.ManageEvent)
	}
	return &Notifier{
		routingKey: routingKey,
		runbookURL: runbookURL,
		client:     client,
		triggered:  map[string]bool{},
	}
}

type clientFunc func(pagerduty.V2Event) (*pagerduty.V2EventResponse, error)

func (f clientFunc) ManageEvent(event pagerduty.V2Event) (*pagerduty.V2EventResponse, error) {
	return f(event)
}

func (d *Notifier) Alert(dedupKey, summary string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.triggered[dedupKey] {
		return
	}
	log.Println("PagerDuty: ", summary)
	event := pagerduty.V2Event{
		Action:     "trigger",
		RoutingKey: d.routingKey,
		DedupKey:   dedupKey,
		Payload: &pagerduty.V2Payload{
			Summary:  summary,
			Source:   dedupKey,
			Severity: "critical",
		},
		Images: []interface{}{
			map[string]string{
				"src": "https://github.com/errm/alertdog/raw/main/docs/dog.jpg",
			},
		},
	}
	if d.runbookURL != "" {
		event.Links = []interface{}{
			map[string]string{
				"text": "Runbook 📕",
				"href": d.runbookURL,
			},
		}
	}
	if _, err := d.client.ManageEvent(event); err != nil {
		log.Printf("Error raising alert on pagerduty %s: %s", dedupKey, err)
		return
	}
	d.triggered[dedupKey] = true
}

func (d *Notifier) Resolve(dedupKey string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.triggered[dedupKey] {
		return
	}
	log.Println("PagerDuty: resolving ", dedupKey)
	event := pagerduty.V2Event{
		Action:     "resolve",
		RoutingKey: d.routingKey,
		DedupKey:   dedupKey,
	}
	if _, err := d.client.ManageEvent(event); err != nil {
		log.Printf("Error resolving alert on pagerduty %s: %s", dedupKey, err)
		return
	}
	d.triggered[dedupKey] = false
}
