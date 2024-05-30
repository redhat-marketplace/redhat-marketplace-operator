The IBM Metrics Operator provides workload metering and reporting for IBM Software Central and Red Hat Marketplace customers.
### **Important Note**
A set of instructions for onboarding is provided here. For more detailed onboarding instructions or information about what is installed please visit [swc.saas.ibm.com](https://swc.saas.ibm.com).

Usage metrics may be monitored through [https://swc.saas.ibm.com](https://swc.saas.ibm.com) with only IBM Metrics Operator and a Red Hat Marketplace account, and does not require Red Hat Marketplace Deployment Operator.

Full cluster registration and software lifecycle management through [https://swc.saas.ibm.com](https://swc.saas.ibm.com) requires both IBM Metrics Operator and Red Hat Marketplace Deployment Operator.

### Upgrade Policy

The operator releases adhere to semantic versioning and provides a seamless upgrade path for minor and patch releases within the current stable channel.

### Prerequisites
1. Installations are required to [enable monitoring for user-defined projects](https://docs.openshift.com/container-platform/latest/monitoring/enabling-monitoring-for-user-defined-projects.html) as the Prometheus provider.
2. Edit the cluster-monitoring-config ConfigMap object:

   ```sh
   $ oc -n openshift-monitoring edit configmap cluster-monitoring-config
    ```

3. Add enableUserWorkload: true under data/config.yaml:
  
    ```sh
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: cluster-monitoring-config
        namespace: openshift-monitoring
    data:
      config.yaml: |
        enableUserWorkload: true
    ```

4. Configure the user-workload-monitoring-config ConfigMap object:

    ```sh
    $ oc -n openshift-user-workload-monitoring edit configmap user-workload-monitoring-config
    ```

5. Configure a minimum retention time of 168h and minimum storage capacity of 40Gi
  
    ```sh
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: user-workload-monitoring-config
      namespace: openshift-user-workload-monitoring

    data:
      config.yaml: |
        prometheus:
          retention: 168h
          volumeClaimTemplate:
            spec:
              resources:
                requests:
                  storage: 40Gi
    ```

### Resources Required

Minimum system resources required:

| Operator                | Memory (MB) | CPU (cores) | Disk (GB) | Nodes |
| ----------------------- | ----------- | ----------- | --------- | ----- |
| **Metrics**   |        750  |     0.25    | 3x1       |    3  |
| **Deployment** |        250  |     0.25    | -         |    1  |

| Prometheus Provider  | Memory (GB) | CPU (cores) | Disk (GB) | Nodes |
| --------- | ----------- | ----------- | --------- | ----- |
| **[Openshift User Workload Monitoring](https://docs.openshift.com/container-platform/latest/monitoring/enabling-monitoring-for-user-defined-projects.html)** |          1  |     0.1       | 2x40        |   2    |

Multiple nodes are required to provide pod scheduling for high availability for IBM Metrics Operator Data Service and Prometheus.

The IBM Metrics Operator automatically creates 3 x 1Gi PersistentVolumeClaims to store reports as part of the data service, with _ReadWriteOnce_ access mode. Te PersistentVolumeClaims are automatically created by the ibm-metrics-operator after creating a `redhat-marketplace-pull-secret` and accepting the license in `marketplaceconfig`.

| NAME                                | CAPACITY | ACCESS MODES |
| ----------------------------------- | -------- | ------------ |
| rhm-data-service-rhm-data-service-0 | 1Gi | RWO |
| rhm-data-service-rhm-data-service-1 | 1Gi | RWO |
| rhm-data-service-rhm-data-service-2 | 1Gi | RWO |

### Supported Storage Providers

- OpenShift Container Storage / OpenShift Data Foundation version 4.x, from version 4.2 or higher
- IBM Cloud Block storage and IBM Cloud File storage
- IBM Storage Suite for IBM Cloud Paks:
  - File storage from IBM Spectrum Fusion/Scale 
  - Block storage from IBM Spectrum Virtualize, FlashSystem or DS8K
- Portworx Storage, version 2.5.5 or above
- Amazon Elastic File Storage

### Access Modes required

 - ReadWriteOnce (RWO)

### Provisioning Options supported

Choose one of the following options to provision storage for the ibm-metrics-operator data-service

#### Dynamic provisioning using a default StorageClass
   - A StorageClass is defined with a `metadata.annotations: storageclass.kubernetes.io/is-default-class: "true"`
   - PersistentVolumes will be provisioned automatically for the generated PersistentVolumeClaims
--- 
#### Manually create each PersistentVolumeClaim with a specific StorageClass
   - Must be performed before creating a `redhat-marketplace-pull-secret` or accepting the license in `marketplaceconfig`. Otherwise, the automatically generated PersistentVolumeClaims are immutable.
```
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  labels:
    app: rhm-data-service
  name: rhm-data-service-rhm-data-service-0
  namespace: ibm-software-central
spec:
  storageClassName: rook-cephfs
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
```
---
#### Manually provision each PersistentVolume for the generated PersistentVolumeClaims with a specific StorageClass
  - May be performed before or after creating a `redhat-marketplace-pull-secret` or accepting the license in `marketplaceconfig`.  
```
apiVersion: v1
kind: PersistentVolume
metadata:
  name: rhm-data-service-rhm-data-service-0
spec:
  csi:
    driver: rook-ceph.cephfs.csi.ceph.com
    volumeHandle: rhm-data-service-rhm-data-service-0
  capacity:
    storage: 1Gi
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Delete
  storageClassName: rook-cephfs
  volumeMode: Filesystem
  claimRef:
    kind: PersistentVolumeClaim
    namespace: ibm-software-central
    name: rhm-data-service-rhm-data-service-0
```

### Installation
1. Create or get your [pull secret](https://swc.saas.ibm.com/en-us/documentation/clusters#get-pull-secret).
2. Install the IBM Metrics Operator
3. Create a Kubernetes secret in the installed namespace with the name `redhat-marketplace-pull-secret` and key `PULL_SECRET` with the value of the pull secret.
    ```sh
    # Replace ${PULL_SECRET} with your pull secret
    oc create secret generic redhat-marketplace-pull-secret -n redhat-marketplace --from-literal=PULL_SECRET=${PULL_SECRET}
    ```
4. Use of the platform is governed by the:

    [IBM Cloud Services Agreement](https://www.ibm.com/support/customer/csol/terms/?id=Z126-6304_WS&_ga=2.116312197.2046730452.1684328846-812467790.1684328846) (or other base agreement between you and IBM such as a [Passport Advantage Agreement](https://www.ibm.com/software/passportadvantage/pa_agreements.html?_ga=2.116312197.2046730452.1684328846-812467790.1684328846)) and the [Service Description for the Red Hat Marketplace](https://www.ibm.com/support/customer/csol/terms/?id=i126-8719&_ga=2.83289621.2046730452.1684328846-812467790.1684328846).
    
5. Update MarketplaceConfig to accept the license.
    ```
    oc patch marketplaceconfig marketplaceconfig -n redhat-marketplace --type='merge' -p '{"spec": {"license": {"accept": true}}}'
    ```
6. Install the pull secret as a global pull secret on the cluster.

    These steps require `oc`, `jq`, and `base64` to be available on your machine.

    ```sh
    # Create the docker pull secret file using your pull secret.
    # Store it in a file called entitledregistryconfigjson.
    oc create secret docker-registry entitled-registry --docker-server=registry.marketplace.redhat.com --docker-username "cp" --docker-password "${PULL_SECRET}" --dry-run=client --output="jsonpath={.data.\.dockerconfigjson}" | base64 --decode > entitledregistryconfigjson
    # Get the current global secrets on the cluster and store it as a file named dockerconfigjson
    oc get secret pull-secret -n openshift-config --output="jsonpath={.data.\.dockerconfigjson}" | base64 --decode > dockerconfigjson
    # Merge the two dockerconfigs together into a file called dockerconfigjson-merged.
    jq -s '.[0] * .[1]' dockerconfigjson entitledregistryconfigjson > dockerconfigjson-merged
    # Set the cluster's dockerconfig file to the new merged version.
    oc set data secret/pull-secret -n openshift-config --from-file=.dockerconfigjson=dockerconfigjson-merged
    ```

### Why is a global pull secret required?
In order to successfully install the products hosted by the container image registry, you will need to make the pull secret available across the cluster. This can be achieved by applying the pull as a [global pull secret](https://docs.openshift.com/container-platform/latest/openshift_images/managing_images/using-image-pull-secrets.html#images-update-global-pull-secret_using-image-pull-secrets). For alternative approaches, please see the official OpenShift [documentation](https://docs.openshift.com/container-platform/latest/openshift_images/managing_images/using-image-pull-secrets.html).


### SecurityContextConstraints requirements

The Operators and their components support running under the OpenShift Container Platform default restricted and restricted-v2 security context constraints.

### Installation Namespace and ClusterRoleBinding requirements

The IBM Metrics Operator components require specific ClusterRoleBindings.
- The metric-state component requires a ClusterRoleBinding for the the `view` ClusterRole.
  - Ability to read non-sensitive CustomResources. 
  - The `view` ClusterRole dynamically adds new CustomResourceDefinitions.
- The operator & reporter component requires a ClusterRoleBinding for the the `cluster-monitoring-view` ClusterRole.
  - Ability to view Prometheus metrics. 
  - The underlying ClusterRole RBAC often updated between OpenShift versions.

Due to limitations of Operator Lifecycle Manager (OLM), this ClusterRoleBinding can not be provided dynamically for arbitrary installation target namespaces.

A static ClusterRoleBinding is included for installation to the default namespace of `redhat-marketplace`, and namespaces `openshift-redhat-marketplace`, `ibm-common-services`.

To create the ClusterRoleBindings for installation to an alternate namespace
```
oc project INSTALL-NAMESPACE
oc adm policy add-cluster-role-to-user view -z ibm-metrics-operator-metric-state
oc adm policy add-cluster-role-to-user cluster-monitoring-view -z ibm-metrics-operator-controller-manager,ibm-metrics-operator-reporter
```

### Metric State scoping requirements
The metric-state Deployment obtains `get/list/watch` access to metered resources via the `view` ClusterRole. For operators deployed using Operator Lifecycle Manager (OLM), permissions are added to `clusterrole/view` dynamically via a generated and annotated `-view` ClusterRole. If you wish to meter an operator, and its Custom Resource Definitions (CRDs) are not deployed through OLM, one of two options are required
1. Add the following label to a clusterrole that has get/list/watch access to your CRD: `rbac.authorization.k8s.io/aggregate-to-view: "true"`, thereby dynamically adding it to `clusterrole/view`
2. Create a ClusterRole that has get/list/watch access to your CRD, and create a ClusterRoleBinding for the metric-state ServiceAccount

Attempting to meter a resource with a MeterDefinition without the required permissions will log an `AccessDeniedError` in metric-state.

### Disaster Recovery

To plan for disaster recovery, note the PhysicalVolumeClaims `rhm-data-service-rhm-data-service-N`. 
- In connected environments, MeterReport data upload attempts occur hourly, and are then removed from data-service. There is a low risk of losing much unreported data.
- In an airgap environment, MeterReport data must be pulled from data-service and uploaded manually using `datactl`. To prevent data loss in a disaster scenario, the data-service volumes should be considered in a recovery plan.

### Subscription Config

It is possible to [configure](https://github.com/operator-framework/operator-lifecycle-manager/blob/master/doc/design/subscription-config.md) how OLM deploys an Operator via the `config` field in the [Subscription](https://github.com/operator-framework/olm-book/blob/master/docs/subscriptions.md) object.

The IBM Metrics Operator will also read the `config` and append it to the operands. The primary use case is to control scheduling using [Tolerations](https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/) and [NodeSelectors](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/).

A limitation is that the `config` elements are only appended to the operands. The elements in the operands are not removed if the `config` is removed from the `Subscripion`. The operand must be modified manually, or deleted and recreated by the controller.

### Cluster permission requirements

|API group             |Resources              |Verbs                                     |
|----------------------|-----------------------|------------------------------------------|
|apps                  |statefulsets           |get;list;watch                            |
|authentication.k8s.io |tokenreviews           |create                                    |
|authorization.k8s.io  |subjectaccessreviews   |create                                    |
|config.openshift.io   |clusterversions        |get;list;watch                            |
|marketplace.redhat.com|meterdefinitions       |get;list;watch;create;update;patch;delete |
|operators.coreos.com  |clusterserviceversions |get;list;watch                            |
|operators.coreos.com  |subscriptions          |get;list;watch                            |
|operators.coreos.com  |operatorgroups         |get;list                                  |
|storage.k8s.io        |storageclasses         |get;list;watch                            |
|                      |namespaces             |get;list;watch                            |
|                      |configmaps             |get;list;watch                            |
|                      |services               |get;list;watch                            |
|                      |**clusterrole/view**   |                                          |


### Documentation
You can find our documentation [here.](https://swc.saas.ibm.com/en-us/documentation/)

### Getting help
If you encounter any issues while using the operators, you can create an issue on our [Github
repo](https://github.com/redhat-marketplace/redhat-marketplace-operator) for bugs, enhancements, or other requests. You can also visit our main page and
review our [support](https://swc.saas.ibm.com/en-us/support) and [documentation](https://swc.saas.ibm.com/en-us/documentation/).

### Readme
You can find our readme [here.](https://github.com/redhat-marketplace/redhat-marketplace-operator/blob/develop/README.md)

### License information
You can find our license information [here.](https://github.com/redhat-marketplace/redhat-marketplace-operator/blob/develop/LICENSE)
