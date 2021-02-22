# Alertdog

![alert dog](docs/dog.jpg "alert dog")

[Guard dog](https://www.flickr.com/photos/_pavan_/5519497579) by [`_paVan_`](https://www.flickr.com/photos/_pavan_/) is licensed under [CC BY 2.0](https://creativecommons.org/licenses/by/2.0/)

## Design

Alertdog is designed to receive webhook notifications of so called "Watchdog"
alerts produced by prometheus instances via an alertmanager cluster.

See [here](https://github.com/prometheus-operator/kube-prometheus/blob/1bf43811174355359e5316b52bfb1a0b928550b2/jsonnet/kube-prometheus/components/mixin/alerts/general.libsonnet#L19-L31) for an example of a watchdog alert.

* If Alertdog doesn't receive an Watchdog alert for a configured prometheus
instance within a configurable expiry time then an alert will be triggered
within alertmanager.
* If  Alertdog doesn't receive any webhook activity for a configurable expiry
time then it raises a PagerDuty incident, as it is assumed that this means that
alertmanager is down.
* If  Alertdog encounters errors triggering alerts on alertmanager then it
raises a PagerDuty incident, as it is assumed that this means that alertmanager
is down.

Alertdog is designed to be used in situations where a single Alertmanager
is used to route alerts from multiple Prometheus instances to different
teams, each with there own alerting setup.

If a Prometheus instance is down, but Alertmanager is still functioning correctly
we want to be able to make use of configured routing and alert methods so that
the owning team can receive notifications in the normal way.

If Alertmanager itself is down we fall back to using a third-party alerting system
PagerDuty, additional targets could be added in the future if required!
