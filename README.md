# dataflow-operator

- An operator demo of which dataflow engine running in kubernetes

---

# Introduction

## Notes

- The webhooks feature is not supported, will be implemented in the future

---

## Environment configuration

- go version not be higher than 1.18,now is 1.17.8. Currently, kubebuilder is not compatible with version 1.18
- kubernetes version is 1.23.0
- docker version is 20.10.14, docker desktop is 4.8.1
- kubebuilder version is 3.3.0
- kustomize version is 4.5.4

---

## USE

### Run the Controller locally

1. Install CRD in the kubernetes

```shell
    make manifests
    
    make install
```

2. Run controller

```shell
    make run
```

3. Test for user etcd standalone

```shell
    kubectl apply -f config/samples/dataflow_v1_user_storage.yaml
```

- Connect to etcd server for test

```shell
    // image:cnych/etcd:v3.4.13
    kubectl exec -it etcd-demo-0 -n dev --  /bin/sh 
    
    // input
   etcdctl --endpoints etcd-demo-0.etcd-demo:2379,etcd-demo-1.etcd-demo:2379,etcd-demo-2.etcd-demo:2379 endpoint status --write-out=table
```

```shell
    // image:quay.io/coreos/etcd
    kubectl exec -it etcd-demo-0 -n dev --  /bin/sh
    
    // input
    etcdctl member list
```

4. Test for frame mysql standalone

- Get CR

```shell
    kubectl apply -f config/samples/dataflow_v1_frame_storage.yaml
```

- Connect to mysql server for test

```shell
    kubectl run -it --rm --image=mysql:5.7 --restart=Never mysql-client -n dev -- mysql -h mysql-service -ppassword
```

5. Verify

```shell
    kubectl get pod -n dev
```

6. Delete CR

```shell
    kubectl delete -f xxx.yaml
```

7. Uninstall CRD

```shell
    make uninstall
```
