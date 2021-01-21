# MeterDefinition Onboarding

## Prerequisites

This onboarding doc has some work involved, if you intend to following along you'll at least need these two prereqs.

- OpenShift CLI (oc) or kubectl
- An OpenShift cluster with Red Hat Marketplace operator installed

## Meter Definition Description

The Meter Definition is the core resource for defining metrics to bill by use. This section describes the field and the concepts so you better understand how to make use of the resource.

### Root

Available Fields

| Field           | Required     | Valid Values              | Description                                                                                                                                       |
| :-------------- | :----------- | :------------------------ | :------------------------------------------------------------------------------------------------------------------------------------------------ |
| group           | yes          | string                    | Group is the domain of your organization. It should match the Group of the CRDs in use or be the domain of your organization.                     |
| kind            | yes          | string                    | Kind describe the type of resource being metered. It should match the CRD Kind or be a descriptive work to organize the meters of the definition. |
| resourceFilters | at least one | array of resource filters | Resource filters provide a means of selecting the resources to monitor on the cluster. Meters will only be to be run against selected resources.  |
| meters          | at least one | array of meters           | Meters are the definition of metrics that are used to collect metric data and build a report.                                                     |

### Resource Filters

Resource filters each have the ability to select a namespace, and filter to a workload type. There are 3 additional filters that at least one must be selected: owner CRD, label, or annotation. These will further filter the workloads to only include the set of resources that should be metered.

The owner CRD, label, and annotation filters can be combined on each resource filter definition to perform an AND. If you wish to have OR, you will need to define another resourceFilter.

Available fields:

| Field        | Required | Valid Values                        | Description                                                                                   |
| :----------- | :------- | :---------------------------------- | :-------------------------------------------------------------------------------------------- |
| namespace    | yes      | object                              | Filter namespace by using operator group                                                      |
| workloadType | yes      | Pod, Service, PersistentVolumeClaim | Filter the resources by the type. Required, a resource filter can only be defined for a type. |
| ownerCRD     | one of   | object                              | Find resources based on the Owner References on the Kubernetes Object metadata.               |
| label        | one of   | object                              | Filter resources by the labels assigned.                                                      |
| annotation   | one of   | object                              | Filter annotations by the annotations on the Kubernetes object metadata.                      |

Example of all the available actions:

```yaml
resourceFilters:
  - namespace:
      useOperatorGroup: true
      labelSelector:
        matchLabels:
          myApp: true
    workloadType: Pod
    ownerCRD:
      apiVersion: marketplace.redhat.com/v1alpha1
      kind: RazeeDeployment
    label:
      labelSelector:
        matchLabels:
          app.kubernetes.io/name: rhm-metric-state
    annotation:
      annotationSelector:
        matchLabels:
          anAnnotation: ['foo']
```

### Meters

Meters are the definition of how to query and build a report to send for metering. It allows customizing of the collection of data to fit different scenarios.

Available fields:

| Field       | Required | Valid Values                 | Description                                                                                                                        |
| :---------- | :------- | :--------------------------- | :--------------------------------------------------------------------------------------------------------------------------------- |
| metric      | yes      | string                       | Metric identifier unique to that metric in the prometheus format (my_metric).                                                      |
| name        | no       | string                       | Common name meant to be used in graph and charts.                                                                                  |
| description | no       | string                       | Common description meant to be used by graphs and charts                                                                           |
| query       | yes      | string                       | Prometheus query to gather the metric data.                                                                                        |
| aggregation | no       | sum,min,max,avg              | Aggregation is the type of final agreggation to perform on the values for the period. Default is sum.                              |
| period      | no       | 1h, 2h, 4h, 6h, 8h, 12h, 24h | Period is the amount of time to block the day's report into. Value must be divisable into 24. Default is 1 hour.                   |
| groupBy     | no       | string slice                 | Prometheus labels to group by, default values for the workload type are used if not set.                                           |
| without     | no       | string slice                 | Prometheus labels to leave out of the result set, all labels are included by default so this helps remove possibly sensitive data. |

## Preconfigured

Before you get started, look at some preconfigured options. These options are pretty much a drop in for any operator and can be used to immediately start metering. You'll want to change these fields to be unique to you:

- metadata.name
- spec.meterGroup
- spec.meterKind

### Max of Pod Count/hr

To take the max pod count, you'll need to replace the created_by fields in the query. Valid values for kind are `daemonset`, `deployment`, `replicaset`.

```yaml
apiVersion: marketplace.redhat.com/v1beta1
kind: MeterDefinition
metadata:
  name: my-operator-max-pod-count
spec:
  group: partner.metering.com # replace with your Group
  kind: App # replace with your Kind
  resourceFilters:
    - namespace:
        useOperatorGroup: true
      ownerCRD:
        apiVersion: partner.metering.com/v1alpha1
        kind: App
      workloadType: Pod
  meters:
    - aggregation: max
      period: 1h
      metricId: pod_count
      workloadType: Pod
      query: |
        min_over_time(
          (kube_pod_info{
            created_by_kind="${KIND_OF_YOUR_DEPLOYMENT}",
            created_by_name="${NAME_OF_YOUR_DEPLOYMENT}"
            node=~".*"} or on() vector(0))[60m:60m])
```

### Sum of vCPU Used

1. Container vCPU on a pod

   ```yaml
   apiVersion: marketplace.redhat.com/v1beta1
   kind: MeterDefinition
   metadata:
     name: my-container-vcpu-use
   spec:
     group: partner.metering.com # replace with your Group
     kind: App # replace with your Kind
     resourceFilters:
       - namespace:
           useOperatorGroup: true
         ownerCRD:
           apiVersion: partner.metering.com/v1alpha1
           kind: App
         workloadType: Pod
     meters:
       - aggregation: sum
         period: 1h
         metricId: container_vcpu_use
         workloadType: Pod
         query: rate(container_cpu_usage_seconds_total{cpu="total", container="${YOURCONTAINER}"}[5m])*100
   ```

1. Track vCPU for the entire workload pod

   ```yaml
   apiVersion: marketplace.redhat.com/v1beta1
   kind: MeterDefinition
   metadata:
     name: my-operator-cpu-use
   spec:
     group: partner.metering.com # replace with your Group
     kind: App # replace with your Kind
     resourceFilters:
       - namespace:
           useOperatorGroup: true
         ownerCRD:
           apiVersion: partner.metering.com/v1alpha1
           kind: App
         workloadType: Pod
     meters:
       - aggregation: sum
         period: 1h
         metricId: pod_vcpu_sum
         workloadType: Pod
         query: sum by (pod, namespace) (rate(container_cpu_usage_seconds_total{container_name!="POD"}[1m])*100)
   ```

#### Query breakdown

The raw metric available to us in prometheus will have a lot of labels. It is best for you to review and decide if what you are tracking is what you want. For instance, a database may have 4 containers. A proxy, an auth helper, a backup tool and the actual database workload. To actively track what the database itself is uing you would want to select that container. Let's say our container names are:

- proxy
- authhelper
- backup
- database

We would change the query to this:

```
rate(container_cpu_usage_seconds_total{cpu="total", container="database"}[30s])*100
```

Let's break down the query:

| Query                                                                               | Description                                                                                                                                                                                                                 |
| :---------------------------------------------------------------------------------- | :-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| container_cpu_usage_seconds_total{cpu="total", container="database"}                | This is querying for container cpu usage seconds total. In Kubernetes, each container is alotted an amount of CPU time. This is the "total" number of seconds that the container "database" used                            |
| container_cpu_usage_seconds_total{cpu="total", container="database"}[1m]            | Now that we have our query, we want want to sample taht so we can get an accurate picture of use. This will sample the data over the last 1 minute.                                                                         |
| rate(container_cpu_usage_seconds_total{cpu="total", container="database"}[1m])      | We take the rate of our 1 minute sample. This will give us the average use over the 1 minute. We use rate because the metric is a counter that can be reset (reset to 0), rate can handle that natively so data isn't lost. |
| rate(container_cpu_usage_seconds_total{cpu="total", container="database"}[1m])\*100 | Up til now, we're working with fractions. This will turn the value into a whole unit. A value of 1 is 1 vCPU core.                                                                                                          |

## Manual

### Steps

Here is a highlevel overview of what we'll be doing to create a MeterDefinition.

1. Create your MeterDefinition base.
2. Choose your vertex type.
3. Identify what you would like to meter? Pod, Service or PersistentVolumeClaim.
4. Create your workload.
5. Create your workload filters.
6. Debug your workload filters.
7. Create your metricLabel queries for your workload.
8. Review your query and test it.
9. Add it to your ClusterServiceVersion file.

### Create your MeterDefinition Base

Create the MeterDefinition base. Select a name for your meter definition that logically makes sense for your product. Your product may have multiple definitions so having an intuitive and unique name is required. The namespace for development purposes should be the same namespace you're developing your operator in. Avoid `default` namespace as a rule.

The first two fields we'll create are meterGroup and meterKind. These fields identify your workloads in our systems and should be unique for your operator. It's best to mirror what your operator CRDs here.

- meterGroup - the domain of your Operator is best here.
- meterKind - the CRD Kind matching your operand is best here.

Here is an example of our App operator for the domain partner.metering.com

```yaml
apiVersion: marketplace.redhat.com/v1beta1
kind: MeterDefinition
metadata:
  name: userCount
  namespace: partner-metering
spec:
  # Add fields here
  group: partner.metering.com
  kind: App
```

### Identify what you would like to meter?

Identify your workload involves looking at the type. Do you want to track a Pod, PersistentVolumeClaim or Service?

Default data sources are [kube-state](https://github.com/kubernetes/kube-state-metrics) and [cadvisor](https://github.com/google/cadvisor/blob/master/metrics/prometheus.go) and can be used to match with your workload to build a query.

For our example we'll use Service, and a custom metric.

### Create your meter

```yaml
apiVersion: marketplace.redhat.com/v1beta1
kind: MeterDefinition
metadata:
  name: userCount
  namespace: partner-metering
spec:
  # Add fields here
  group: partner.metering.com
  kind: App
  meters:
    - aggregation: sum
      period: 1h
      metricId: app_customer-metric
      query: custom_metric{}
      workloadType: Service
```

### Create your resfilters

Your options for workload filters are as follows: Owner Custom Resource Definition (CRD) API Version, Annotation, or Labels. Any combination of the 3 are available. We'll use OperatorGroup for the rest of the example.

- Use Owner CRD API Version

  ```yaml
  apiVersion: marketplace.redhat.com/v1beta1
  kind: MeterDefinition
  metadata:
    name: userCount
    namespace: partner-metering
  spec:
    # Add fields here
    group: partner.metering.com
    kind: App
    resourceFilters:
      - namespace:
          useOperatorGroup: true
        ownerCRD:
          apiVersion: partner.metering.com/v1alpha1
          kind: App
        workloadType: Service
    meters:
      - aggregation: sum
        period: 1h
        metricId: app_customer-metric
        query: custom_metric{}
        workloadType: Service
  ```

### Refine your meter

Meter queries are probably the most difficult part of creating a MeterDefinition. The process requires some trial and error and knowledge of Prometheus is a plus. If this is difficult for you, please reach out to our team.

Here is an example to find an imaginary user count from a service, returning a sum of all the data points.

```yaml

  apiVersion: marketplace.redhat.com/v1beta1
  kind: MeterDefinition
  metadata:
    name: userCount
    namespace: partner-metering
  spec:
    # Add fields here
    group: partner.metering.com
    kind: App
    resourceFilters:
      - namespace:
          useOperatorGroup: true
        ownerCRD:
          apiVersion: partner.metering.com/v1alpha1
          kind: App
        workloadType: Service
    meters:
      - aggregation: max
        period: 1h
        metricId: app_customer-metric
        query: custom_metric{}
        query: rate(container_cpu_usage_seconds_total{cpu="total", container="${YOURCONTAINER}"}[5m])*100
        workloadType: Service
```

### Debug your workload filters.

Apply your meterdefinition to the cluster. And you can then inspect these things to verify it is working correctly.

- Is it finding the correct workloads?

  On the MeterDefinition status, there will be a list of workload objects discovered. Use this to verify if it's finding all the resources you're looking for.

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

All data collected by MeterDefinitions are stored in Prometheus. We'll create a fake meter report for the last hour, use a CLI tool and the prometheus port-forwarded locally to debug.

1. Install the [Red Hat Marketplace Operator](https://marketplace.redhat.com/en-us/documentation/getting-started).
1. Install your meter definition.
1. Port forward to prometheus for your local host.
   ```sh
   kubectl port-forward -n openshift-redhat-marketplace prometheus-rhm-marketplaceconfig-meterbase-0 9090:9090
   ```
1. Create your test meterreport. You'll want to change the start and end time to match the current date.

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
   ```

1. Run your meter report using the reporter helper tool.

   ```sh
   reporter report --name test-meter-report --namespace openshift-redhat-marketplace --upload=false --zap-devel
   ```

1. Use the stdout to debug your MeterDefinition.

   - You'll see a line like this:

     ```share
     "logger":"reporter","msg":"output","query":
     ```

     Beside query will be a string containing the query for the MeterDefinition. You can directly access Prometheus and see what the results are. The query is long, but it's what is used to deliver the final result. If there is a prometheus syntax error, it's likely an issue with your query. Try to run it first by itself.

1. Advanced Troubleshooting

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

## Deploying the Meter Definition

### Adding to CSV

The Red Hat Marketplace operator supports adding the meter definition to the Cluster Service Version (CSV) as an annotation. Export your meter definition as a json and add it to the CSV before your create your bundle before Red Hat certification. When your operator is installed, it will be applied from the CSV to the cluster and automatically installed.

```yaml
type: operators/ClusterServiceVersion
metadata:
  name: YourCSV
  annotations:
    marketplace.redhat.com/meterDefinition: |
      // json of meter definition
```

Or you can create the meter definition on the cluster via APIs.
