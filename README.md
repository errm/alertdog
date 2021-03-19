![Alertdog: Watching over your alerting system; Barks if it breaks.](docs/dog.jpg "woof 🐕 - alertmanager is broken")

[Guard dog](https://www.flickr.com/photos/_pavan_/5519497579) by [`_paVan_`](https://www.flickr.com/photos/_pavan_/) is licensed under [CC BY 2.0](https://creativecommons.org/licenses/by/2.0/)

Alertdog is software system to detect failures in a prometheus + alertmanager
alerting system.

If there is a problem that means that prometheus, or alertmanager are not working
as expected Alertdog will raise an alert, either via alertmanager, or if that
is not possible via PagerDuty.

It is designed specifically to meet the needs of an organisation [Cookpad](https://www.cookpadteam.com/) where
several Prometheus clusters are managed by different teams, but
a single alertmanager cluster is utilised.

## Design

You can read detailed information about the design of Alertdog [here](docs/design.md)

## Getting started

To get started with Alertdog check out the [getting started documentation](docs/getting_started.md)

## Installation

Alertdog is released as a docker image to [docker hub](https://hub.docker.com/r/errm/alertdog), [quay.io](https://quay.io/repository/errm/alertdog?tab=tags) and [aws ecr](https://gallery.ecr.aws/w5g2l5d0/alertdog)

Find details of the released tags [here](https://github.com/errm/alertdog/releases)

We recommend that you run alertdog with a container orchestrator like Kubernetes
see an example [here](example/alertdog.yml)

## Config File Format

`config.yml`

```yaml
# A list of alertmanager endpoints (used when pushing alerts to alertmanager)
alertmanager_endpoints:
  - http://alertmanager-0:9093
  - http://alertmanager-1:9093


# How often Alertdog checks if a Watchdog has been recieved from an expected
# prometheus (optional) (defaults to 2m)
check_interval: 2m

# How long Alertdog will wait before raising a PagerDuty alert if no webhook
# requests were recieved from alertmanger (optional) (defaults to 5m)
expiry: 5m

# The port that the webhook endpoint is exposed on (optional) (defaults to 9767)
port: 9767

# A PagerDuty EventsV2 API routing key
pager_duty_key: PAGER_DUTY_KEY

# A url for a runbook, to be included in PagerDuty alerts (optional)
pagerduty_runbook_url: https://example.org/alertmanager_down_runbook

# A list of prometheus clusters that we expect to recieve Watchdog alerts from
expected:
  -
    # The labels that match this prometheus cluster, should be set to the
    # Alertname you use for watchdog alerts, and any external labels set on
    # this cluster. Note you need to make sure that your cluster have unique labels.
    match_labels:
      alertname: Watchdog
      owner: team-a

    # How long to wait after reciving a watchdog before raising an alert (optional) (defaults to 4m)
    # Note this value should be longer than `check_interval`.
    # Make sure that alertmanager repeats the alert at least this often,
    # by setting `repeat_interval` & `group_interval` to appropriate values in
    # the alertmanger route configuration see example/alertmanger.yml for an example of this.
    expiry: 4m

    # The configuration of the alert that will be raised in alertmanager if the
    # Watchdog isn't recieved within the configured expiry time
    alert:
      name: PrometheusAlertFailure
      labels:
        severity: critical
        owner: team-a
        service: monitoring
        component: prometheus
      annotations:
        runbook: https://example.org/prometheus-alert-failure
        description: |
          This alert fires when the "Watchdog" alert fails to be received
          from a prometheus instance.
          It could indicate that the prometheus instance is not running, or
          that there is a configuration issue with prometheus or alertmanager.
```

## Contributing

* If you find a bug please raise an issue.
* If it's security related please contact me on: edward-robinson@cookpad.com [GPG key here](https://keybase.io/errm)

* PRs are welcome :)
* If you open a PR from your own fork the Test and Lint github actions should be working
* The repo also has a PR action that attempts to push a built container image to Docker Hub
  * This action is so you / I can do some manual testing if required!
  * Your fork won't have the secret with credentials for my docker hub :)
  * If you want it to work on your fork:
    * Create an repo in your DockerHub, set it's name in the `DOCKERHUB_PR_REPO` on your fork's secrets.
    * Create a DockerHub token that can push to that repo
    * Set the `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` secrets on your fork
