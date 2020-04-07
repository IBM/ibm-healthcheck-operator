
# ibm-healthcheck-operator

Operator used to manage the IBM system healthcheck service.

## Supported platforms

- Red Hat OpenShift Container Platform 4.2
- Red Hat OpenShift Container Platform 4.3

## Operating systems

Linux® x86_64

## Operator versions

3.5.0

## Prerequisites

- go version v1.13+.
- Docker version 17.03+
- Kubectl v1.11.3+
- Access to a Kubernetes v1.11.3+ cluster

## Documentation

For installation and configuration, see the [IBM Cloud Platform Common Services documentation](http://ibm.biz/cpcsdocs).

## Getting Started

### Cloning the repository

Check out the ibm-healthcheck-operator repository.

```bash
# git clone https://github.com/IBM/ibm-healthcheck-operator.git
# cd ibm-healthcheck-operator
```

### Building the operator

Build the ibm-healthcheck-operator image and push it to a public registry, such as quay.io.

```bash
# make images
```

### Using the image

Edit `deploy/operator.yaml` and replace the image name.

```bash
vim deploy/operator.yaml
```

### Installing

```bash
# kubectl apply -f deploy/
deployment.apps/ibm-healthcheck-operator created
role.rbac.authorization.k8s.io/ibm-healthcheck-operator created
clusterrole.rbac.authorization.k8s.io/ibm-healthcheck-operator-cluster created
rolebinding.rbac.authorization.k8s.io/ibm-healthcheck-operator created
clusterrolebinding.rbac.authorization.k8s.io/ibm-healthcheck-operator-cluster created
serviceaccount/ibm-healthcheck-operator created
serviceaccount/ibm-healthcheck-operator-cluster created
```

```bash
# kubectl get pods
NAME                                          READY   STATUS    RESTARTS   AGE
ibm-healthcheck-operator-75976c8fc-f9vts      1/1     Running   0          62s
icp-memcached-74657c849f-8l4v4                1/1     Running   0          33s
system-healthcheck-service-6bd476b58f-ffc4s   1/1     Running   0          32s
```

### Uninstalling

```bash
# kubectl delete -f deploy/
```

### Installing ODLM

[Install ODLM](https://github.com/IBM/operand-deployment-lifecycle-manager/blob/master/docs/install/install.md) in your cluster to help manage all the operators.

### Troubleshooting

Use the following command to check the operator logs.

```bash
# kubectl logs deployment.apps/ibm-healthcheck-operator -n <namespace>
```
