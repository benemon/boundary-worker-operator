# boundary-worker-operator
This Operator deploys an HCP Boundary Worker onto Kubernetes.

## Description
The Boundary Worker Operator only supports Boundary's Controller-Led Authorisation flow. Before deploying, two configuration elements are required:

* HCP Boundary Cluster ID - the long alphanumeric value in your HCP Boundary url
* Controller Generated Activation Token

In order to generate a new token before creating the BoundaryPKIWorker CustomResources:

```sh
$ boundary workers create controller-led -name=my-worker

Worker information:
  Active Connection Count:                 0
  Controller-Generated Activation Token:   neslat_..
  Created Time:                            Tue, 26 Sep 2023 10:09:35 BST
  ID:                                      w_wWl6LJ0viX
  Name:                                    worker-operator
  Type:                                    pki
  Updated Time:                            Tue, 26 Sep 2023 10:09:35 BST
  Version:                                 1

  Scope:
    ID:                                    global
    Name:                                  global
    Type:                                  global

  Authorized Actions:
    no-op
    read
    update
    delete
    add-worker-tags
    set-worker-tags
    remove-worker-tags

```

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster

### Deploy controller
Deploy the controller from the cluster:

On Kubernetes:
```sh
make deploy
```

On OpenShift:
```sh
make oc-deploy
```

### Create the CR

```yaml
apiVersion: workers.boundaryproject.io/v1alpha1
kind: BoundaryPKIWorker
metadata:
  labels:
    app.kubernetes.io/name: boundarypkiworker
    app.kubernetes.io/instance: boundarypkiworker-sample
    app.kubernetes.io/part-of: boundary-worker-operator
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/created-by: boundary-worker-operator
  name: boundarypkiworker-sample
spec:
  registration:
    controllerGeneratedActivationToken: neslat_... # required on first run; can be removed after if desired
    hcpBoundaryClusterID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx # required - defines the HCP Boundary cluster to communicate with
  resources: # optional - permits the declaration of runtime resources as required
    requests: # optional - standard Kubernetes resource requests
      cpu: # optional
      memory: # optional
    limits # optional - standard Kubernetes resource limits
      cpu: # optional
      memory: # optional
    storage: # optional - can be used to override the cluster storage configuration
      storageClassName: managed-csi # optional - can be used to override the default storageclass if required
  tags: #optional - map of custom tags to add to the Boundary Worker configuration. Changes in tags triggers a new rollout.
     key1: value1
     key2: value2, value3, value4 # comma seperated tags are supported, and will be split into discreet values for the given key
     key3: value5

```

Then create the CR in the cluster:

```sh
$ kubectl apply -f boundarypkiworker-sample.yaml
```

### Outcome
The Operator will create several resources:

* StatefulSet - Worker deployment, maintaining a 1-1 mapping with a PersistentVolume
* ConfigMap - Worker configuration
* Headless Service - required for the StatefulSet
* PersistentVolume - stores Worker state

Once deployed, and assuming the Worker is configured correctly, the Worker will register with the HCP Cluster. In addition to any custom tags you may supply in the CR, the Worker will be configured with a number of default, Kubernetes specific tags:

```sh
tags {
    	type = ["kubernetes"]
		namespace = ["application"]
		boundary_pki_worker = ["worker1"]
	}
```

* type - Kubernetes
* namespace - The Namespace the Worker is deployed in
* boundary_pki_worker -the name of the worker, derived from the CR

### Undeploy controller
UnDeploy the controller from the cluster:

On Kubernetes:
```sh
make undeploy
```

On OpenShift:
```sh
make oc-undeploy
```
## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

