# Meter Definition Onboarding

## Prerequisites

- Openshift CLI (oc) or kubectl
- An OpenShift cluster with Red Hat Marketplace Metering enabled

## Preconfigured

Before you get started. Look at some preconfigured options here:

### Max of Pod Count/hr

TBD

```yaml
apiVersion: marketplace.redhat.com/v1alpha1
kind: MeterDefinition
metadata:
  name: max-pod-count
spec:
  # Add fields here
  meterGroup: partner.metering.com # replace this
  meterKind: App # replace this
  workloadVertexType: OperatorGroup
  workloads:
    - name: app-pods
      type: Pod
```

### Sum of vCPU Used

TBD

```yaml
apiVersion: marketplace.redhat.com/v1alpha1
kind: MeterDefinition
metadata:
  name: example-meterdefinition-3
spec:
  # Add fields here
  meterGroup: partner.metering.com
  meterKind: App
  workloadVertexType: OperatorGroup
  workloads:
    - name: app-pods
      type: Pod
```

## Manual

### Steps

Here is a highlevel overview of what we'll be doing to create a meter definition.

1. Create your meter definition base.
2. Choose your vertex type.
3. Identify what you would like to meter? Pod, Service or PersistentVolumeClaim.
4. Create your workload.
5. Create your workload filters.
6. Debug your workload filters.
7. Create your metricLabel queries for your workload.
8. Review your query and test it.
9. Add it to your ClusterServiceVersion file.

### Create your Meter Definition Base

Create the meter definition base. This will have a name that logically makes sense for your product. Your product can technically have multiple definitions so it will matter. The namespace for development purposes should be the same namespace you're developing your operator in. Avoid `default` namespace as a rule.

The first two fields we'll create are. These fields identify your workloads in our systems and should be unique for your operator. It's best to mirror what your operator CRDs here.

- meterGroup - the domain of your Operator is best here.
- meterKind - the CRD Kind matching your operand is best here.

Here is an example of our App operator for the domain partner.metering.com

```yaml
apiVersion: marketplace.redhat.com/v1alpha1
kind: MeterDefinition
metadata:
  name: userCount
  namespace: partner-metering
spec:
  # Add fields here
  meterGroup: partner.metering.com
  meterKind: App
```

### Choose your vertex type.

Meter definitions are anchored by the vertex. This is the place to start to look for your workloads. There are two options available: OperatorGroup or Namespace with a selector. Unless you have a very specific reason not to use OperatorGroup, you should always use OperatorGroup.

```yaml
apiVersion: marketplace.redhat.com/v1alpha1
kind: MeterDefinition
metadata:
  name: userCount
  namespace: partner-metering
spec:
  # Add fields here
  meterGroup: partner.metering.com
  meterKind: App
  workloadVertexType: OperatorGroup
```

### Identify what you would like to meter?

Identify your workload involves looking at the type. Do you want to track a Pod, PersistentVolumeClaim or Service?

Default data sources are [kube-state](https://github.com/kubernetes/kube-state-metrics) and [cadvisor](https://github.com/google/cadvisor/blob/master/metrics/prometheus.go) and can be used to match with your workload to build a query.

For our example we'll use Service, and a custom metric.

### Create your workload.

The workload is the logical block of work

```yaml
apiVersion: marketplace.redhat.com/v1alpha1
kind: MeterDefinition
metadata:
  name: userCount
  namespace: partner-metering
spec:
  # Add fields here
  meterGroup: partner.metering.com
  meterKind: App
  workloadVertexType: OperatorGroup
  workloads:
    - name: user-count
      type: Service
      ownerCRD:
        apiVersion: partner.metering.com/v1alpha1
        kind: App
      metricLabels:
        - label: container_spec_cpu_shares
          aggregation: sum
```

### 5. Create your workload filters.

Your options for workload filters are as follows: Owner Custom Resource Definition (CRD) API Version, Annotation, or Labels. Any combination of the 3 are available. We'll use OperatorGroup for the rest of the example but

- Use Owner CRD API Version

  ```yaml
  apiVersion: marketplace.redhat.com/v1alpha1
  kind: MeterDefinition
  metadata:
    name: userCount
    namespace: partner-metering
  spec:
    # Add fields here
    meterGroup: partner.metering.com
    meterKind: App
    workloadVertexType: OperatorGroup
    workloads:
      - name: user-count
        type: Service
        ownerCRD:
          apiVersion: partner.metering.com/v1alpha1
          kind: App
  ```

- Use Annotations

  ```yaml
  apiVersion: marketplace.redhat.com/v1alpha1
  kind: MeterDefinition
  metadata:
    name: userCount
    namespace: partner-metering
  spec:
    # Add fields here
    meterGroup: partner.metering.com
    meterKind: App
    workloadVertexType: OperatorGroup
    workloads:
      - name: user-count
        type: Service
        annotationSelector: #
          matchLabels:
            parnet.metering.com/product.id: abas342341321-12341451
  ```

- Use Labels

  ```yaml
  apiVersion: marketplace.redhat.com/v1alpha1
  kind: MeterDefinition
  metadata:
    name: userCount
    namespace: partner-metering
  spec:
    # Add fields here
    meterGroup: partner.metering.com
    meterKind: App
    workloadVertexType: OperatorGroup
    workloads:
      - name: user-count
        type: Service
        labelSelector:
          matchLabels:
            app-id: AppSimple
  ```

- Use any combination. At least one is required but you can use any combination to achieve your goal.

  ```yaml
  apiVersion: marketplace.redhat.com/v1alpha1
  kind: MeterDefinition
  metadata:
    name: userCount
    namespace: partner-metering
  spec:
    # Add fields here
    meterGroup: partner.metering.com
    meterKind: App
    workloadVertexType: OperatorGroup
    workloads:
      - name: user-count
        type: Service
        ownerCRD:
          apiVersion: partner.metering.com/v1alpha1
          kind: App
        annotationSelector: #
          matchLabels:
            parnet.metering.com/product.id: abas342341321-12341451
        labelSelector:
          matchLabels:
            app-id: AppSimple
  ```

### Create your metric label queries for your workload.

Metric Label queries are probably the most difficult part of creating a meter definition. The process requires some trial and error and knowledge of Prometheus is a plus. If this is difficult for you, please reach out to our team.

| Field       | Description                                                                                                                  |
| :---------- | :--------------------------------------------------------------------------------------------------------------------------- |
| label       | The name of the result, it's the human readable label for the customer.                                                      |
| query       | The Prometheus query to use on your workload type (Service, Pod, PVC)                                                        |
| aggregation | Each query is calculated for an hour, for a day, and data points are aggregated. Valid values are `sum`, `min`, `max`, `avg` |

Here is an example to find an imaginary user count from a service, returing a sum of all the data points.

```yaml
apiVersion: marketplace.redhat.com/v1alpha1
kind: MeterDefinition
metadata:
  name: userCount
  namespace: partner-metering
spec:
  # Add fields here
  meterGroup: partner.metering.com
  meterKind: App
  workloadVertexType: OperatorGroup
  workloads:
    - name: user-count
      type: Service
      ownerCRD:
        apiVersion: partner.metering.com/v1alpha1
        kind: App
      metricLabels:
        - label: user_count
          query: my_user_count{service_name="simple-service"}
          aggregation: max
```

### Debug your workload filters.

Apply your meterdefinition to the cluster. And you can then inspect these things to verify it is working correctly.

- Is it finding the correct workloads?

  On the meter definition status, there will be a list of workload objects discovered. Use this to verify if it's finding all the resources you're looking for.

- I am using Service but it can't be found.

  Services are a little special. For the Owner CRD option to work correctly there needs to be the correct ownership.

  For the Services to be picked up by OwnerCRD there needs to be a ServiceMonitor and the ownership should follow this:

  ```
  ServiceMonitor <- owned by Service (SetOwnerReference)
  Service <- owned by your controller (SetControllerReference)
  ```

  For more information on ownership look at the [Ownership Ref object](https://github.com/kubernetes/apimachinery/blob/master/pkg/apis/meta/v1/types.go#L300) and the controllerutil helper functions [SetControllerReference](https://godoc.org/sigs.k8s.io/controller-runtime/pkg/controller/controllerutil#SetControllerReference) and [SetOwnerReference](https://godoc.org/sigs.k8s.io/controller-runtime/pkg/controller/controllerutil#SetOwnerReference).

  Additionally, make sure your ServiceMonitor is being picked up by the Prometheus for metering in the next step.

### Review your query and test it.

**Note:** Features to improve this step are currently being worked so this can be a bit difficult to debug on your own.

All data collected by meter definitions are stored in Prometheus. We'll create a fake meter report for the last hour, use a CLI tool and the prometheus port-forwarded locally to debug.

1. Install the [Red Hat Marketplace Operator](https://marketplace.redhat.com/en-us/documentation/getting-started).
2. Port forward to prometheus for your local host.
   ```sh
   kubectl port-forward -n openshift-redhat-marketplace prometheus-rhm-marketplaceconfig-meterbase-0 9090:9090
   ```
3. Create your test meterreport. You'll want to change the start and end time to match the current date. Insert your meter definition in the meter report under meterDefinitions field.

   ```yaml
   apiVersion: marketplace.redhat.com/v1alpha1
   kind: MeterReport
   metadata:
     name: test-meter-report
     namespace: openshift-redhat-marketplace
   spec:
     startTime: '2020-10-19T00:00:00Z'
     endTime: '2020-10-20T00:00:00Z'
     prometheusService:
       bearerTokenSecret:
         key: ''
       name: rhm-prometheus-meterbase
       namespace: openshift-redhat-marketplace
       targetPort: rbac
     meterDefinitions:
       - meterGroup: partner.metering.com
         meterKind: App
         workloadVertexType: OperatorGroup
         workloads:
           - name: user-count
             type: Service
             ownerCRD:
               apiVersion: partner.metering.com/v1alpha1
               kind: App
             metricLabels:
               - label: user_count
                 query: my_user_count{service_name="simple-service"}
                 aggregation: max
   ```

4. Run your meter report using the reporter helper tool.

   ```sh
   reporter report --name test-meter-report --namespace openshift-redhat-marketplace --upload=false --zap-devel
   ```

5. Use the stdout to debug your meter definition.

   - You'll see a line like this:

     ```share
     "logger":"reporter","msg":"output","query":
     ```

     Beside query will be a string containing the query for the Meter definition. You can directly access Prometheus and see what the results are. The query is long, but it's what is used to deliver the final result. If there is a prometheus syntax error, it's likely an issue with your query. Try to run it first by itself.

6. Advanced Troubleshooting

   - Query is working but returns no results

     This can happen for a lot of reasons. The primary reason is that your query is not returning results that can be matched up for the resource. A basic case of monitoring pods needs a pod name and a namespace to get a final result to match on.

     For example, let's look at the query for tracking number of pods. We will not want to count pods that are replacing other pods so we need to take a `min_over_time` to only grab the pods that existed for the majority of an hour.

     ```
     min_over_time(
       (kube_pod_info{
         created_by_kind="DaemonSet",
         created_by_name="<NAME_OF_YOUR_DAEMONSET>"
         node=~".*"} or on() vector(0))[60m:60m])
     ```

     This result, when ran, will return a prometheus row like this:

     ```
      {created_by_kind="DaemonSet",created_by_name="machine-config-daemon",job="kube-state-metrics",namespace="openshift-machine-config-operator",pod="machine-config-daemon-z9sln",service="kube-state-metrics"} 1
     ```

     The final result has pod and namespace, that is a good sign. Aggregations can strip fields and leave results blank. If that occurs, then you'll need to tune the query until it returns the right values.

     The final query can take a sum and accurately get a count of active pods being used by our Daemonset.
