# `NamespaceClass` Operator
A Kubernetes Operator that manages the lifecycle of a `NamespaceClass` resource. Built using [kubebuilder](https://book.kubebuilder.io/).

## Overview
The `NamespaceClass` Operator is designed to automate the management of `NamespaceClass` resources in a Kubernetes
cluster. A `NamespaceClass` defines a set of resources and policies that can be applied to namespaces within the cluster.
When a `Namespace` is labeled with a specific `NamespaceClass`, the operator ensures that the resources and policies
defined in the `NamespaceClass` are applied to that namespace.

## Features
* Automatically creates and manages resources defined in a `NamespaceClass` for each namespace that uses it
* Support for defining any resource type within a `NamespaceClass`
* Switching `NamespaceClass` for a namespace will automatically update the resources in that namespace
* Updating a `NamespaceClass` will automatically update the resources in any namespace using that class

## Getting Started
This project is in the early stages of development. Here's how you can get started using and developing locally.

### Prerequisites
- Go 1.20 or later
- Kubernetes cluster (minikube, kind, etc.)
- `kubectl`
- `kubebuilder`

### Running Locally
With your Kubernetes cluster running, you can run the operator locally using the following commands:

1. Generate `WebhookConfiguration`, `ClusterRole` and `CustomResourceDefinition` objects
```bash
make manifests
```

2. Generate code containing `DeepCopy`, `DeepCopyInto`, and `DeepCopyObject` method implementations
```bash
make generate
```

3. Install CRDs into the K8s cluster specified in `~/.kube/config`
```bash
make install
```

4. Run the controller from your host
```bash
make run
```

5. Apply a `NamespaceClass` CRD to the cluster
Example CRD:
```yaml
apiVersion: stanton.sh/v1
kind: NamespaceClass
metadata:
  labels:
    app.kubernetes.io/name: namespace-class
  name: namespaceclass-sample
spec:
  resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: configmap-sample
      namespace: default
    - apiVersion: v1
      kind: Secret
      metadata:
        name: secret-sample
    - apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: deployment-sample
      spec:
        replicas: 1
        selector:
          matchLabels:
            app: deployment-sample
        template:
          metadata:
            labels:
              app: deployment-sample
          spec:
            containers:
              - name: nginx
                image: nginx:latest
                ports:
                  - containerPort: 80
```

```bash
kubectl apply -f config/samples/nsclass-sample.yaml
```

6. Create a `Namespace` and label it with the `NamespaceClass`
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: sample
  labels:
    namespaceclass.stanton.sh/name: namespaceclass-sample
```

```bash
kubectl apply -f config/samples/ns-sample.yaml
```

7. Verify that the resources defined in the `NamespaceClass` have been created in the `sample` namespace
```bash
kubectl get configmap,secret,deployment -n sample
NAME                         DATA   AGE
configmap/configmap-sample   0      54s
configmap/kube-root-ca.crt   1      54s

NAME                   TYPE     DATA   AGE
secret/secret-sample   Opaque   0      54s

NAME                                READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/deployment-sample   1/1     1            1           54s
```
