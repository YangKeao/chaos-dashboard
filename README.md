# chaos-dashboard

## Usage

```
helm install helm/chaos-dashboard --name=chaos-dashboard --namespace=chaos-dashboard
kubectl port-forward -n chaos-dashboard svc/chaos-dashboard 8080:80
```

Then you can access chaos-dashboard on [localhost:8080](http://localhost:8080)