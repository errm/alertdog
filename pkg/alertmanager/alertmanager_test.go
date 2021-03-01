package alertmanager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/prometheus/alertmanager/client"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

func TestAlert(t *testing.T) {
	var (
		errc             = make(chan error, 1)
		expected         = make([]*client.Alert, 0, 1)
		status1, status2 atomic.Int32
		slow1, slow2     atomic.Bool
	)

	status1.Store(int32(http.StatusOK))
	status2.Store(int32(http.StatusOK))

	newHTTPServer := func(status *atomic.Int32, slow *atomic.Bool, checkAlerts func([]*client.Alert, []*client.Alert) error) *httptest.Server {

		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var err error
			defer func() {
				if err == nil {
					return
				}
				select {
				case errc <- err:
				default:
				}
			}()
			var alerts []*client.Alert

			err = json.NewDecoder(r.Body).Decode(&alerts)
			if err == nil {
				err = checkAlerts(expected, alerts)
			}
			s := int(status.Load())
			w.WriteHeader(s)
			if slow.Load() {
				time.Sleep(11 * time.Second)
			}
			if s == http.StatusOK {
				w.Write([]byte("{\"status\":\"success\"}"))
			} else {
				w.Write([]byte("{\"status\":\"error\"}"))
			}
		}))
	}

	server1 := newHTTPServer(&status1, &slow1, alertsOK)
	server2 := newHTTPServer(&status2, &slow2, alertsOK)

	defer server1.Close()
	defer server2.Close()

	alertManager := Alertmanager{
		Endpoints: []string{
			server1.URL,
			server2.URL,
		},
		Expiry: time.Minute,
	}

	checkNoErr := func() {
		t.Helper()
		select {
		case err := <-errc:
			require.NoError(t, err)
		default:
		}
	}

	expected = append(expected, &client.Alert{
		Labels: toLabelSet(map[string]string{
			"alertname": "PrometheusAlertFailure",
			"foo":       "bar",
		}),
	})

	// Both servers OK
	require.NoError(t, alertManager.Alert(Alert{
		Name: "PrometheusAlertFailure",
		Labels: map[string]string{
			"foo": "bar",
		},
	}), "Alerting failed unexpectedly")
	checkNoErr()

	// Only one server erring
	status2.Store(int32(http.StatusInternalServerError))
	require.NoError(t, alertManager.Alert(Alert{
		Name: "PrometheusAlertFailure",
		Labels: map[string]string{
			"foo": "bar",
		},
	}), "Alerting failed unexpectedly")
	checkNoErr()

	// Both servers error
	status1.Store(int32(http.StatusNotFound))
	require.Error(t, alertManager.Alert(Alert{
		Name: "PrometheusAlertFailure",
		Labels: map[string]string{
			"foo": "bar",
		},
	}), "Alerting succeeded unexpectedly")
	checkNoErr()

	//Timeout
	status1.Store(int32(http.StatusOK))
	status2.Store(int32(http.StatusOK))
	slow1.Store(true)
	slow2.Store(true)
	require.Error(t, alertManager.Alert(Alert{
		Name: "PrometheusAlertFailure",
		Labels: map[string]string{
			"foo": "bar",
		},
	}), "Alerting succeeded unexpectedly")
	checkNoErr()

	//Dead server
	server1.Close()
	server2.Close()
	require.Error(t, alertManager.Alert(Alert{
		Name: "PrometheusAlertFailure",
		Labels: map[string]string{
			"foo": "bar",
		},
	}), "Alerting succeeded unexpectedly")

	// Resolve
	server1 = newHTTPServer(&status1, &slow1, resolveOK)
	server2 = newHTTPServer(&status2, &slow2, resolveOK)
	defer server1.Close()
	defer server2.Close()

	alertManager = Alertmanager{
		Endpoints: []string{
			server1.URL,
			server2.URL,
		},
	}

	status1.Store(int32(http.StatusOK))
	status2.Store(int32(http.StatusOK))
	slow1.Store(false)
	slow2.Store(false)

	require.NoError(t, alertManager.Resolve(Alert{
		Name: "PrometheusAlertFailure",
		Labels: map[string]string{
			"foo": "bar",
		},
	}), "Alerting succeeded unexpectedly")
}

func alertsOK(expected, actual []*client.Alert) error {
	if len(expected) != len(actual) {
		return fmt.Errorf("length mismatch: %v != %v", expected, actual)
	}
	for i, alert := range expected {
		if !labelsEqual(alert.Labels, actual[i].Labels) {
			return fmt.Errorf("label mismatch at index %d: %s != %s", i, alert.Labels, actual[i].Labels)
		}
	}
	for _, alert := range actual {
		if alert.EndsAt != alert.StartsAt.Add(time.Minute) {
			return fmt.Errorf("Expected EndsAt to be %s was %s", alert.StartsAt.Add(time.Minute), alert.EndsAt)
		}
	}
	return nil
}

func resolveOK(expected, actual []*client.Alert) error {
	if len(expected) != len(actual) {
		return fmt.Errorf("length mismatch: %v != %v", expected, actual)
	}
	for i, alert := range expected {
		if !labelsEqual(alert.Labels, actual[i].Labels) {
			return fmt.Errorf("label mismatch at index %d: %s != %s", i, alert.Labels, expected[i].Labels)
		}
	}
	for _, alert := range actual {
		if alert.EndsAt != alert.StartsAt {
			return fmt.Errorf("Expected EndsAt to equal StartsAt  %s vs %s", alert.EndsAt, alert.StartsAt)
		}
	}
	return nil
}

func labelsEqual(a, b client.LabelSet) bool {
	return reflect.DeepEqual(a, b)
}
