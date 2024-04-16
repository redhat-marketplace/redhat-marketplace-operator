The Red Hat Marketplace Deployment Operator by IBM provides cluster and operator management for Red Hat Marketplace customers.
### **Important Note**
A set of instructions for onboarding is provided here. For more detailed onboarding instructions or information about what is installed please visit [swc.saas.ibm.com](https://swc.saas.ibm.com).

Full registration and visibility of usage metrics on [https://swc.saas.ibm.com](https://swc.saas.ibm.com) requires both IBM Metrics Operator and Red Hat Marketplace Deployment Operator.

### Upgrade Policy

The operator releases adhere to semantic versioning and provides a seamless upgrade path for minor and patch releases within the current stable channel.

### Prerequisites

1. The Red Hat Markeplace Deployment Operator prerequisites the IBM Metrics Operator. Installing the Red Hat Markeplace Deployment Operator with Automatic approval on the Subscription will also install the IBM Metrics Operator automatically. If performing an install with Manual approval, install the IBM Metrics Operator first.

#### The IBM Metrics Operator prequisites the following

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

### Installation
1. Create or get your pull secret from [Red Hat Marketplace](https://swc.saas.ibm.com/en-us/documentation/clusters#get-pull-secret).
2. Install the IBM Metrics Operator and Red Hat Marketplace Deployment Operator
3. Create a Kubernetes secret in the installed namespace with the name `redhat-marketplace-pull-secret` and key `PULL_SECRET` with the value of the Red hat Marketplace Pull Secret.
    ```sh
    # Replace ${PULL_SECRET} with your secret from Red Hat Marketplace
    oc create secret generic redhat-marketplace-pull-secret -n  redhat-marketplace --from-literal=PULL_SECRET=${PULL_SECRET}
    ```
4. Use of the Red Hat Marketplace platform is governed by the:

    [IBM Cloud Services Agreement](https://www.ibm.com/support/customer/csol/terms/?id=Z126-6304_WS&_ga=2.116312197.2046730452.1684328846-812467790.1684328846) (or other base agreement between you and IBM such as a [Passport Advantage Agreement](https://www.ibm.com/software/passportadvantage/pa_agreements.html?_ga=2.116312197.2046730452.1684328846-812467790.1684328846)) and the [Service Description for the Red Hat Marketplace](https://www.ibm.com/support/customer/csol/terms/?id=i126-8719&_ga=2.83289621.2046730452.1684328846-812467790.1684328846).

5. Update MarketplaceConfig to accept the license.
    ```
    oc patch marketplaceconfig marketplaceconfig -n redhat-marketplace --type='merge' -p '{"spec": {"license": {"accept": true}}}'
    ```
6. Install the pull secret as a global pull secret on the cluster.

    These steps require `oc`, `jq`, and `base64` to be available on your machine.

    ```sh
    # Create the docker pull secret file using your PULL_SECRET from IBM Software Central.
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

### Subscription Config

It is possible to [configure](https://github.com/operator-framework/operator-lifecycle-manager/blob/master/doc/design/subscription-config.md) how OLM deploys an Operator via the `config` field in the [Subscription](https://github.com/operator-framework/olm-book/blob/master/docs/subscriptions.md) object.

The Red Hat Marketplace Deployment Operator will also read the `config` and append it to the operands. The primary use case is to control scheduling using [Tolerations](https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/) and [NodeSelectors](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/).

A limitation is that the `config` elements are only appended to the operands. The elements in the operands are not removed if the `config` is removed from the `Subscripion`. The operand must be modified manually, or deleted and recreated by the controller.

### Cluster permission requirements

|API group             |Resources                 |Verbs                             |
|----------------------|--------------------------|----------------------------------|
|apiextensions.k8s.io  |customresourcedefinitions |get;list;watch                    |
|apps                  |deployments               |get;list;watch                    |
|apps                  |replicasets               |get;list;watch                    |
|authentication.k8s.io |tokenreviews              |create                            |
|authorization.k8s.io  |subjectaccessreviews      |create                            |
|config.openshift.io   |clusterversions           |get;list;watch                    |
|config.openshift.io   |consoles                  |get;list;watch                    |
|config.openshift.io   |infrastructures           |get;list;watch                    |
|marketplace.redhat.com|marketplaceconfigs        |get;list;watch                    |
|marketplace.redhat.com|remoteresources3s         |get;list;watch                    |
|deploy.razee.io       |remoteresources           |get;list;watch                    |
|operators.coreos.com  |catalogsources            |create;get;list;watch;delete      |
|operators.coreos.com  |clusterserviceversions    |get;list;watch;update;patch;delete|
|operators.coreos.com  |operatorgroups            |get;list;watch;delete;create      |
|operators.coreos.com  |subscriptions             |*                                 |
|                      |configmaps                |get;list;watch                    |
|                      |namespaces                |get;list;watch                    |
|                      |nodes                     |get;list;watch                    |
|                      |pods                      |get;list;watch                    |


### Documentation
You can find our documentation [here.](https://swc.saas.ibm.com/en-us/documentation/)

### Getting help
If you encounter any issues while using Red Hat Marketplace operator, you can create an issue on our [Github
repo](https://github.com/redhat-marketplace/redhat-marketplace-operator) for bugs, enhancements, or other requests. You can also visit our main page and
review our [support](https://swc.saas.ibm.com/en-us/support) and [documentation](https://swc.saas.ibm.com/en-us/documentation/).

### Readme
You can find our readme [here.](https://github.com/redhat-marketplace/redhat-marketplace-operator/blob/develop/README.md)

### License information
You can find our license information [here.](https://github.com/redhat-marketplace/redhat-marketplace-operator/blob/develop/LICENSE)
