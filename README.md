![Alertdog: Watching over your alerting system; Barks if it breaks.](docs/dog.jpg "woof 🐕 - alertmanager is broken")

[Guard dog](https://www.flickr.com/photos/_pavan_/5519497579) by [`_paVan_`](https://www.flickr.com/photos/_pavan_/) is licensed under [CC BY 2.0](https://creativecommons.org/licenses/by/2.0/)

Alertdog is software system to detect failures in a prometheus + alertmanager
alerting system.

If there is problem that means that prometheus, or alertmanager are not working
as expected Alertdog will raise an alert, either via alertmanager, or if that
is not possible via PagerDuty.

It is designed specifically to meet the needs of an organisation (Cookpad)[https://www.cookpadteam.com/] where
several Prometheus clusters are managed by different teams, but
a single alertmanager cluster is utilised.

## Design

You can read detailed information about the design of Alertdog [here](docs/design.md)

## Getting started

To get started with Alertdog check out the [getting started documentation](docs/getting_started.md)

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
