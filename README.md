
# IBM Health Check Operator

## Overview

IBM Health Check Operator is used to manage the IBM health check service

## Prerequisites

- go version v1.13+.
- docker version 17.03+
- kubectl v1.11.3+
- Access to a Kubernetes v1.11.3+ cluster

## Getting Started

### Cloning the repository

Checkout this IBM Health Check Operator repository

```bash
# git clone https://github.com/IBM/ibm-healthcheck-operator.git
# cd ibm-healthcheck-operator
```

### Building the operator

Build the ibm-healthcheck-operator image and push it to a public registry, such as quay.io:

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

### Install from OLDM

[Install OLDM](https://github.com/IBM/operand-deployment-lifecycle-manager/blob/master/docs/install/install.md) in your cluser, and let OLDM help manage all the operators.

### Troubleshooting

Use the following command to check the operator logs.

```bash
# kubectl logs deployment.apps/ibm-healthcheck-operator -n <namespace>
```
