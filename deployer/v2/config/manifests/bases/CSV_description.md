The Red Hat Marketplace Deployment Operator by IBM provides cluster and operator management for Red Hat Marketplace customers.
### **Important Note**
A set of instructions for onboarding is provided here. For more detailed onboarding instructions or information about what is installed please visit [marketplace.redhat.com](https://marketplace.redhat.com).

### **Upgrade Notice**

The Red Hat Marketplace Operator metering and deployment functionality have been separated into two operators.
  - The deployment functionality remains as part of this Red Hat Marketplace Deployment Operator
    - The Red Hat Marketplace Deployment Operator prerequisites the IBM Metrics Operator
    - Admin level functionality and permissions are required for deployment functionality
    - ClusterServiceVersion/redhat-marketplace-operator
  - The metering functionality is included in the IBM Metrics Operator
    - Admin level functionality and permissions are removed from the IBM Metrics Operator
    - ClusterServiceVersion/ibm-metrics-operator

Full registration and visibility of usage metrics on [https://marketplace.redhat.com](https://marketplace.redhat.com) requires both IBM Metrics Operator and Red Hat Marketplace Deployment Operator.

### Prerequisites

1. The Red Hat Markeplace Deployment Operator prerequisites the IBM Metrics Operator. Installing the Red Hat Markeplace Deployment Operator with Automatic approval on the Subscription will also install the IBM Metrics Operator automatically. If performing an install with Manual approval, install the IBM Metrics Operator first.

#### The IBM Metrics Operator prequisites the following

1. Installations are required to [enable monitoring for user-defined projects](https://docs.openshift.com/container-platform/4.12/monitoring/enabling-monitoring-for-user-defined-projects.html) as the Prometheus provider.
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
1. Create or get your pull secret from [Red Hat Marketplace](https://marketplace.redhat.com/en-us/documentation/clusters#get-pull-secret).
2. Install the IBM Metrics Operator and Red Hat Marketplace Deployment Operator
3. Create a Kubernetes secret in the installed namespace with the name `redhat-marketplace-pull-secret` and key `PULL_SECRET` with the value of the Red hat Marketplace Pull Secret.

    ```sh
    # Replace ${PULL_SECRET} with your secret from Red Hat Marketplace
    oc create secret generic redhat-marketplace-pull-secret -n  redhat-marketplace --from-literal=PULL_SECRET=${PULL_SECRET}
    ```

4. Install the Red Hat Marketplace pull secret as a global pull secret on the cluster.

    These steps require `oc`, `jq`, and `base64` to be available on your machine.

    ```sh
    # Create the docker pull secret file using your PULL_SECRET from Red Hat Marketplace.
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
In order to successfully install the Red Hat Marketplace products, you will need to make the pull secret available across the cluster. This can be achieved by applying the Red Hat Marketplace Pull Secret as a [global pull secret](https://docs.openshift.com/container-platform/4.12/openshift_images/managing_images/using-image-pull-secrets.html#images-update-global-pull-secret_using-image-pull-secrets). For alternative approachs, please see the official OpenShift [documentation](https://docs.openshift.com/container-platform/4.12/openshift_images/managing_images/using-image-pull-secrets.html).


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
|operators.coreos.com  |catalogsources            |create;get;list;watch;delete      |
|operators.coreos.com  |clusterserviceversions    |get;list;watch;update;patch;delete|
|operators.coreos.com  |operatorgroups            |get;list;watch;delete;create      |
|operators.coreos.com  |subscriptions             |*                                 |
|                      |configmaps                |get;list;watch                    |
|                      |namespaces                |get;list;watch                    |
|                      |nodes                     |get;list;watch                    |
|                      |pods                      |get;list;watch                    |


### Documentation
You can find our documentation [here.](https://marketplace.redhat.com/en-us/documentation/)

### Getting help
If you encounter any issues while using Red Hat Marketplace operator, you can create an issue on our [Github
repo](https://github.com/redhat-marketplace/redhat-marketplace-operator) for bugs, enhancements, or other requests. You can also visit our main page and
review our [support](https://marketplace.redhat.com/en-us/support) and [documentation](https://marketplace.redhat.com/en-us/documentation/).

### Readme
You can find our readme [here.](https://github.com/redhat-marketplace/redhat-marketplace-operator/blob/develop/README.md)

### License information
You can find our license information [here.](https://github.com/redhat-marketplace/redhat-marketplace-operator/blob/develop/LICENSE)
