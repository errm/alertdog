package alertdog

import (
	"testing"
	"time"

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

	alertdog := Alertdog{Expected: []*Prometheus{prom1, prom2}}

	var tests = []struct {
		description  string
		expectations []expectation
		watchdogs    []template.Alert
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
	}

	for _, test := range tests {
		alertmanagerMock := &AlertmanagerMock{}
		alertdog.alertmanager = alertmanagerMock
		for _, expectation := range test.expectations {
			alertmanagerMock.On(expectation.method, expectation.arg).Return(expectation.err)
		}
		for _, watchdog := range test.watchdogs {
			alertdog.processWatchdog(watchdog)
		}
		alertmanagerMock.AssertExpectations(t)
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

	var tests = []struct {
		description  string
		expectations []expectation
		watchdogs    []template.Alert
	}{
		{
			description: "If no watchdogs are received, then fire all alerts",
			expectations: []expectation{
				expectation{method: "Alert", arg: alert1},
				expectation{method: "Alert", arg: alert2},
			},
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
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			alertmanagerMock := &AlertmanagerMock{}

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
				alertmanager: alertmanagerMock,
			}

			for _, expectation := range test.expectations {
				alertmanagerMock.On(expectation.method, expectation.arg).Return(expectation.err)
			}

			for _, watchdog := range test.watchdogs {
				alertdog.processWatchdog(watchdog)
			}

			alertdog.Check()
			alertmanagerMock.AssertExpectations(t)
		})
	}
}
