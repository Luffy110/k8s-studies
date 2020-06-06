## Build code
Using following two commands to get 3pp denpendences and build code.

```shell
go mod vendor
go build -o sample-scheduler -ldflags "-extldflags -static"
```

## Build image
Using folowing command to build docker images.

```shell
docker build -t sample-scheduler:test1 .
```

## Deploy sample-scheduler
Using this command to depoloy custom scheduler.

```shell
kubectl apply -f deploy/sample-scheduler.yaml
```

## Deploy test
Using this command to deploy a test pod to verify custom scheduler.

```shell
kubectl apply -f deploy/test.yaml
```

## Uninstall 

```shell
kubectl delete -f deploy/test.yaml
kubectl delete -f deploy/sample-scheduler.yaml
```