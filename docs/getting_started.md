You can find a full example setup in the `/example` directory.

You can deploy this example to any prometheus cluster e.g. minikube, docker desktop etc...

1. Start by deploying the example:

```
kubectl apply -k example
```

This will deploy 2 prometheus clusters, and a single (2 node) alertmanager cluster, and alertdog.

```
$ kubectl -n alertdog-example get pods
NAME                        READY   STATUS    RESTARTS   AGE
alertdog-6b686c7cd7-jh599   1/1     Running   0          8s
alertmanager-0              1/1     Running   0          8s
alertmanager-1              1/1     Running   0          7s
prometheus-a-0              1/1     Running   0          8s
prometheus-b-0              1/1     Running   0          8s
```

2. Check alertmanager

```
kubectl -n alertdog-example port-forward alertmanager-0 9093
```

Check [alertmanager](http://localhost:9093)

You should see a `Watchdog` alert for each cluster - they are being routed to
alertdog.

3. Kill a prometheus cluster

Remove 1 prometheus to simulate it's failure

```
kubectl -n alertdog-example delete sts prometheus-a
```

4. Check for an alert

After the prometheus watchdog alert expires (Around 3 minutes by default) you
should see a new `PrometheusAlertFailure` in alertmanager.

5. Fix the broken cluster

```
kubectl apply -k example
```

6. Check the alerts

You should see the `Watchdog` alert come back after prometheus has started,
The `PrometheusAlertFailure` alert should be resolved in about a minute.

7. Broken alertmanager

If alertmanager is broken, an incident is raised on PagerDuty.

In order for this to work you need to create an [Events API Key](https://support.pagerduty.com/docs/generating-api-keys#events-api-keys) for your PagerDuty service.

edit `example/alertdogc-config.yml` and replace `PAGER_DUTY_KEY` with your key.

Then if alertmanager is broken an incident should be raised after 5m.

```
kubectl -n alertdog-example delete sts alertmanager
```
