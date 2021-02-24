package alertdog

import (
	"errors"
	"testing"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/alertmanager/template"
	"github.com/stretchr/testify/mock"

	"github.com/errm/alertdog/pkg/alertmanager"
)

type AlertmanagerMock struct {
	mock.Mock
}

func (a AlertmanagerMock) Alert(alert alertmanager.Alert) error {
	args := a.Called(alert)
	return args.Error(0)
}

func (a AlertmanagerMock) Resolve(alert alertmanager.Alert) error {
	args := a.Called(alert)
	return args.Error(0)
}

type PagerdutyMock struct {
	mock.Mock
}

func (p PagerdutyMock) ManageEvent(event pagerduty.V2Event) (*pagerduty.V2EventResponse, error) {
	args := p.Called(event)
	response := &pagerduty.V2EventResponse{}
	return response, args.Error(0)
}

type expectation struct {
	method string
	arg    interface{}
	err    error
}

func TestProcessWatchdog(t *testing.T) {
	alert1 := alertmanager.Alert{
		Labels: map[string]string{
			"alert": "one",
		},
	}

	prom1 := &Prometheus{
		MatchLabels: map[string]string{
			"alertname":  "Watchdog",
			"prometheus": "prom1",
		},
		Alert: alert1,
	}

	alert2 := alertmanager.Alert{
		Labels: map[string]string{
			"alert": "two",
		},
	}

	prom2 := &Prometheus{
		MatchLabels: map[string]string{
			"alertname":  "Watchdog",
			"prometheus": "prom2",
		},
		Alert: alert2,
	}

	alertdog := Alertdog{Expected: []*Prometheus{prom1, prom2}, PagerDutyKey: "pagerduty-key", PagerDutyRunbookURL: "https://example.org/runbook-url"}

	error := errors.New("alertmanager is broken")

	pagerDutyEvent := pagerduty.V2Event{
		Action:     "trigger",
		RoutingKey: "pagerduty-key",
		DedupKey:   "alertdog:alertmanager-push",
		Payload: &pagerduty.V2Payload{
			Summary:  "Alertdog cannot push alerts to alertmanager",
			Source:   "alertdog:alertmanager-push",
			Severity: "critical",
		},
		Images: []interface{}{
			map[string]string{
				"src": "https://github.com/errm/alertdog/raw/main/docs/dog.jpg",
			},
		},
		Links: []interface{}{
			map[string]string{
				"text": "Runbook 📕",
				"href": "https://example.org/runbook-url",
			},
		},
	}

	var tests = []struct {
		description           string
		expectations          []expectation
		watchdogs             []template.Alert
		pagerdutyExpectations []expectation
	}{
		{
			description:  "When we receive a resolved watchdog: alert with the correct alert",
			expectations: []expectation{expectation{method: "Alert", arg: alert1}},
			watchdogs: []template.Alert{
				template.Alert{
					Status: "resolved",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom1",
					},
				},
			},
		},
		{
			description:  "Fire the correct alert based on labels",
			expectations: []expectation{expectation{method: "Alert", arg: alert2}},
			watchdogs: []template.Alert{
				template.Alert{
					Status: "resolved",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom2",
					},
				},
			},
		},
		{
			description:  "Don't care about extra labels",
			expectations: []expectation{expectation{method: "Alert", arg: alert2}},
			watchdogs: []template.Alert{
				template.Alert{
					Status: "resolved",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom2",
						"foo":        "bar",
					},
				},
			},
		},
		{
			description: "Don't do anything with watchdogs that don't match",
			watchdogs: []template.Alert{
				template.Alert{
					Status: "resolved",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom33",
					},
				},
				template.Alert{
					Status: "firing",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom33",
					},
				},
				template.Alert{
					Status: "firing",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom33",
					},
				},
			},
		},
		{
			description:  "When we receive a firing watchdog twice: resolve the correct alert",
			expectations: []expectation{expectation{method: "Resolve", arg: alert1}},
			watchdogs: []template.Alert{
				template.Alert{
					Status: "firing",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom1",
					},
				},
				template.Alert{
					Status: "firing",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom1",
					},
				},
			},
		},
		{
			description:           "When alertmanager errors, raise a pagerduty event",
			expectations:          []expectation{expectation{method: "Alert", arg: alert1, err: error}},
			pagerdutyExpectations: []expectation{expectation{method: "ManageEvent", arg: pagerDutyEvent}},
			watchdogs: []template.Alert{
				template.Alert{
					Status: "resolved",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom1",
					},
				},
			},
		},
	}

	for _, test := range tests {
		alertmanagerMock := &AlertmanagerMock{}
		alertdog.alertmanager = alertmanagerMock
		for _, expectation := range test.expectations {
			alertmanagerMock.On(expectation.method, expectation.arg).Return(expectation.err)
		}

		pagerdutyMock := &PagerdutyMock{}
		alertdog.pagerduty = pagerdutyMock

		for _, expectation := range test.pagerdutyExpectations {
			pagerdutyMock.On(expectation.method, expectation.arg).Return(expectation.err)
		}

		for _, watchdog := range test.watchdogs {
			alertdog.processWatchdog(watchdog)
		}

		alertmanagerMock.AssertExpectations(t)
		pagerdutyMock.AssertExpectations(t)
	}
}

func TestCheck(t *testing.T) {
	alert1 := alertmanager.Alert{
		Labels: map[string]string{
			"alert": "one",
		},
	}

	alert2 := alertmanager.Alert{
		Labels: map[string]string{
			"alert": "two",
		},
	}

	pagerDutyEvent := pagerduty.V2Event{
		Action:     "trigger",
		RoutingKey: "this-is-a-key",
		DedupKey:   "alertdog:webhook-expiry",
		Payload: &pagerduty.V2Payload{
			Summary:  "Alertdog: didn't receive webhook from alert manager for over 2m0s",
			Source:   "alertdog:webhook-expiry",
			Severity: "critical",
		},
		Images: []interface{}{
			map[string]string{
				"src": "https://github.com/errm/alertdog/raw/main/docs/dog.jpg",
			},
		},
	}

	pagerDutyResolveEvent := pagerduty.V2Event{
		Action:     "resolve",
		RoutingKey: "this-is-a-key",
		DedupKey:   "alertdog:webhook-expiry",
	}

	var tests = []struct {
		description           string
		expectations          []expectation
		pagerdutyExpectations []expectation
		watchdogs             []template.Alert
	}{
		{
			description: "If no watchdogs are received, then fire all alerts, and raise a pagerduty incident",
			expectations: []expectation{
				expectation{method: "Alert", arg: alert1},
				expectation{method: "Alert", arg: alert2},
			},
			pagerdutyExpectations: []expectation{expectation{method: "ManageEvent", arg: pagerDutyEvent}},
		},
		{
			description: "Fire the alert if the watchdog was missing",
			expectations: []expectation{
				expectation{method: "Alert", arg: alert1},
			},
			watchdogs: []template.Alert{
				template.Alert{
					Status: "firing",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom2",
					},
				},
			},
			pagerdutyExpectations: []expectation{expectation{method: "ManageEvent", arg: pagerDutyResolveEvent}},
		},
		{
			description: "Don't fire if watchdogs were received",
			watchdogs: []template.Alert{
				template.Alert{
					Status: "firing",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom2",
					},
				},
				template.Alert{
					Status: "firing",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom1",
					},
				},
			},
			pagerdutyExpectations: []expectation{expectation{method: "ManageEvent", arg: pagerDutyResolveEvent}},
		},
		{
			description: "Fire if only resolves where received",
			expectations: []expectation{
				expectation{method: "Alert", arg: alert1},
				expectation{method: "Alert", arg: alert2},
			},
			watchdogs: []template.Alert{
				template.Alert{
					Status: "resolved",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom2",
					},
				},
				template.Alert{
					Status: "resolved",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom1",
					},
				},
			},
			pagerdutyExpectations: []expectation{expectation{method: "ManageEvent", arg: pagerDutyResolveEvent}},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			alertmanagerMock := &AlertmanagerMock{}
			pagerdutyMock := &PagerdutyMock{}

			alertdog := Alertdog{
				Expected: []*Prometheus{
					&Prometheus{
						MatchLabels: map[string]string{
							"alertname":  "Watchdog",
							"prometheus": "prom1",
						},
						Alert:  alert1,
						Expiry: time.Minute,
					},
					&Prometheus{
						MatchLabels: map[string]string{
							"alertname":  "Watchdog",
							"prometheus": "prom2",
						},
						Alert:  alert2,
						Expiry: time.Minute,
					},
				},
				PagerDutyKey: "this-is-a-key",
				Expiry:       time.Minute * 2,
				pagerduty:    pagerdutyMock,
				alertmanager: alertmanagerMock,
			}

			for _, expectation := range test.expectations {
				alertmanagerMock.On(expectation.method, expectation.arg).Return(expectation.err)
			}

			for _, expectation := range test.pagerdutyExpectations {
				pagerdutyMock.On(expectation.method, expectation.arg).Return(expectation.err)
			}

			for _, watchdog := range test.watchdogs {
				alertdog.processWatchdog(watchdog)
			}

			alertdog.Check()
			alertmanagerMock.AssertExpectations(t)
			pagerdutyMock.AssertExpectations(t)
		})
	}
}
