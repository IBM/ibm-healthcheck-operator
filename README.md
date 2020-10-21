# ibm-healthcheck-operator

> **Important:** Do not install this operator directly. Only install this operator using the IBM Common Services Operator. For more information about installing this operator and other Common Services operators, see [Installer documentation](http://ibm.biz/cpcs_opinstall). If you are using this operator as part of an IBM Cloud Pak, see the documentation for that IBM Cloud Pak to learn more about how to install and use the operator service. For more information about IBM Cloud Paks, see [IBM Cloud Paks that use Common Services](http://ibm.biz/cpcs_cloudpaks).

You can use the ibm-healthcheck-operator to install the IBM System Healthcheck service. You can use IBM System Healthcheck service to check the service status of the IBM Cloud Paks and IBM Cloud Platform Common Services.

For more information about the available IBM Cloud Platform Common Services, see the [IBM Knowledge Center](http://ibm.biz/cpcsdocs).

## Supported platforms

Red Hat OpenShift Container Platform 4.3 or newer installed on one of the following platforms:

- Linux x86_64
- Linux on Power (ppc64le)
- Linux on IBM Z and LinuxONE

## Operator versions

- 3.7.2
    - Support for OpenShift 4.3, 4.4 and 4.5.
- 3.7.1
    - Support for OpenShift 4.3, 4.4 and 4.5.
- 3.7.0
    - Support for OpenShift 4.3, 4.4 and 4.5.
- 3.6.1
    - Support for OpenShift 4.3 and 4.4.
- 3.6.0
    - Support for OpenShift 4.3 and 4.4.
- 3.5.0

## Prerequisites

Before you install this operator, you need to first install the operator dependencies and prerequisites:

- For the list of operator dependencies, see the IBM Knowledge Center [Common Services dependencies documentation](http://ibm.biz/cpcs_opdependencies).
- For the list of prerequisites for installing the operator, see the IBM Knowledge Center [Preparing to install services documentation](http://ibm.biz/cpcs_opinstprereq).

## Documentation

To install the operator with the IBM Common Services Operator follow the the installation and configuration instructions within the IBM Knowledge Center.

- If you are using the operator as part of an IBM Cloud Pak, see the documentation for that IBM Cloud Pak. For a list of IBM Cloud Paks, see [IBM Cloud Paks that use Common Services](http://ibm.biz/cpcs_cloudpaks).
- If you are using the operator with an IBM Containerized Software, see the IBM Cloud Platform Common Services Knowledge Center [Installer documentation](http://ibm.biz/cpcs_opinstall).

## SecurityContextConstraints Requirements

The IBM System Healthcheck service supports running with the OpenShift Container Platform 4.3 default restricted Security Context Constraints (SCCs).

For more information about the OpenShift Container Platform Security Context Constraints, see [Managing Security Context Constraints](https://docs.openshift.com/container-platform/4.3/authentication/managing-security-context-constraints.html).

## Developer guide

If, as a developer, you are looking to build and test this operator to try out and learn more about the operator and its capabilities, you can use the following developer guide. This guide provides commands for a quick install and initial validation for running the operator.

> **Important:** The following developer guide is provided as-is and only for trial and education purposes. IBM and IBM Support does not provide any support for the usage of the operator with this developer guide. For the official supported install and usage guide for the operator, see the the IBM Knowledge Center documentation for your IBM Cloud Pak or for IBM Cloud Platform Common Services.

### Quick start guide

Use the following quick start commands for building and testing the operator:

#### Cloning the repository

Check out the ibm-healthcheck-operator repository.

```bash
# git clone https://github.com/IBM/ibm-healthcheck-operator.git
# cd ibm-healthcheck-operator
```

#### Building the operator

Build the ibm-healthcheck-operator image and push it to a public registry, such as quay.io.

```bash
# make images
```

#### Using the image

Edit `deploy/operator.yaml` and replace the image name.

```bash
vim deploy/operator.yaml
```

#### Installing

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

#### Uninstalling

```bash
# kubectl delete -f deploy/
```

### Debugging guide

Use the following commands to debug the operator:

```bash
# kubectl logs deployment.apps/ibm-healthcheck-operator -n <namespace>
```

### End-to-End testing

For more instructions on how to run end-to-end testing with the Operand Deployment Lifecycle Manager, see [ODLM guide](https://github.com/IBM/operand-deployment-lifecycle-manager/blob/master/docs/install/install.md).
