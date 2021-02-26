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
	)

	status1.Store(int32(http.StatusOK))
	status2.Store(int32(http.StatusOK))

	newHTTPServer := func(status *atomic.Int32) *httptest.Server {
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
				err = alertsEqual(expected, alerts)
			}
			s := int(status.Load())
			if s == http.StatusOK {
				w.WriteHeader(s)
				w.Write([]byte("{\"status\":\"success\"}"))
			} else if s == 999 {
				time.Sleep(12 * time.Second)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("{\"status\":\"success\"}"))
			} else {
				w.WriteHeader(s)
				w.Write([]byte("{\"status\":\"error\"}"))
			}
		}))
	}

	server1 := newHTTPServer(&status1)
	server2 := newHTTPServer(&status2)
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
	status1.Store(int32(999))
	status2.Store(int32(999))
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
}

func alertsEqual(a, b []*client.Alert) error {
	if len(a) != len(b) {
		return fmt.Errorf("length mismatch: %v != %v", a, b)
	}
	for i, alert := range a {
		if !labelsEqual(alert.Labels, b[i].Labels) {
			return fmt.Errorf("label mismatch at index %d: %s != %s", i, alert.Labels, b[i].Labels)
		}
	}
	return nil
}

func labelsEqual(a, b client.LabelSet) bool {
	return reflect.DeepEqual(a, b)
}
