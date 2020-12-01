# Meter Definition v1alpha2 Proposal

## Summary

This proposal has use cases for an enhanced meter definition version 2 schema that will be simpler to understand and able to generically handle complex use cases.

## Motivation

### Goals

- Simplify meter definitions
- Make dynamic fields available

### Nongoals

- Backwards compatitibility. v1alpha1 will still exist but will no longer be in use and will be removed slowly.

## Proposal

### User Stories

#### Restructure the meter definition

Meter definitions will be made flatter to better support real world use.

Current implementation is pretty hierarchal. We'll flatten that structure to make it simpler to write and understand.

```yaml
apiVersion: marketplace.redhat.com/v1alpha1
kind: MeterDefinition
metadata:
  name: example-meterdefinition-2
  namespace: metering-example-operator
spec:
  # Add fields here
  meterGroup: partner.metering.com
  meterKind: App

  workloadVertexType: OperatorGroup
  workloads: ## Necessary?
    - name: app-pods
      type: Pod
      ownerCRD:
        apiVersion: partner.metering.com/v1alpha1
        kind: App
      metricLabels: ## this is what we really care able
        - label: container_spec_cpu_shares
          aggregation: sum
    - name: service
      type: Service
      ownerCRD:
        apiVersion: partner.metering.com/v1alpha1
        kind: App
      metricLabels:
        - label: service_count
          aggregation: sum
```

The new proposed structure will flatten and simplify. Vertex is removed for the filter concept where multiple filters are added together instead of indepentent fields. If you'd need to create multiple queries you'd generate multiple meter definitions now.

```yaml
apiVersion: marketplace.redhat.com/v1alpha2
kind: MeterDefinition
metadata:
  name: example-meterdefinition-2
  namespace: metering-example-operator
spec:
  resourceFilter: # a few would be required to be a valid meter definition
    - type: operatorGroup # operatorGroup or namespace required
      useOperatorGroup: true
    - type: resourceKind
      kind: Service
    - type: ownerCRD
      apiVersion: partner.metering.com/v1alpha1
      kind: App
  aggregation: max
  period: 1h
  group: partner.metering.com
  kind: App
  name: Product License usage
  description: Usage of the product
  metric: virtual_processor_core
  groupBy: metric, product_id
  query: >-
    my_metric_data{}
---
# more meter definitions
```

#### Use go templates to add dyanmic labels for better metric reporting

The current meter definitions allow us to monitor and report using a static definition. Instead we can use go templates to dynamically change fields.

For example:

```yaml
apiVersion: marketplace.redhat.com/v1alpha2
kind: MeterDefinition
metadata:
  name: example-meterdefinition-2
  namespace: metering-example-operator
spec:
  resourceFilter:
    - type: operatorGroup # operatorGroup or namespace required
      useOperatorGroup: true
    - type: resourceKind # optional, would default to our default ones to look for
      kind: Service
    - type: ownerCRD
      apiVersion: partner.metering.com/v1alpha1
      kind: App
  aggregation: max
  period: 1h
  group: ${product_id}.partner.metering.com
  kind: ${kind}
  label: ${metric_id}
  name: ${metric} # optional
  metric: ${product_metric} # VIRTUAL_PROCESS_CORE or w/e other things we monitor here
  description: ${description} #optional
  groupBy: metric, product_id
  query: >-
    product_license_usage{}
---
# more meter definitions
```

In this example, we use label results from the query to inject context into the report generated from the meter definition. In this case we could have one meter definition to handle a dynamic prometheus endpoint.

### Support constant metrics


```yaml
apiVersion: marketplace.redhat.com/v1alpha2
kind: MeterDefinition
metadata:
  name: example-meterdefinition-2
  namespace: metering-example-operator
spec:
  resourceFilter:
    - type: operatorGroup # operatorGroup or namespace required
      useOperatorGroup: true
    - type: resourceKind # optional, would default to our default ones to look for
      kind: Service
    - type: ownerCRD
      apiVersion: partner.metering.com/v1alpha1
      kind: App
  aggregation: max
  period: 1h
  group: ${product_id}.partner.metering.com
  kind: ${kind}
  label: ${metric_id}
  name: ${metric} # optional
  metric: ${product_metric} # VIRTUAL_PROCESS_CORE or w/e other things we monitor here
  description: ${description} #optional
  groupBy: metric, product_id
  date: ${date} # SUPPORT DATE VALUE OVERRIDE FROM LABEL FOR CONSTANT DATE VALUES **
  value: ${value} # ** SUPPORT VALUE OVERRIDE FROM LABEL FOR CONSTANT VALUES
  query: >-
    product_license_usage{}
---
# more meter definitions
```


## Design Details

### Test plan

1. Create a dumby endpoint mocking a complex data query.
2. Test the meter definition is able to generate correct reports with and without dynamic labels.

### Upgrade/downgrade strategy

We plan to only support upgrade.

We'll use admission webhooks to migrate from v1alpha1 to v1alpha2, where if there are multiple queries multiple meter definitions will be generated.

Downgrade will not be available to v1alpha1.

## Example

```
product_license_usage{ #cloudpak metric - top level
metric_type=license,
cloudpak_id=[cloudpak_id],
cloudpak_name=[cloudpak_name],
cloudpak_metric=[cloudpak_metric],
[other parameters]} [METRIC_VALUE]
```

```
product_license_usage_details{ #breakdown of cloudpak into subproducts
metric_type=license,
product_id=[product_id], #lasdslfkjaslfja //uuid
product_name=[product_name],
product_metric=[product_metric], #VIRTUAL_PROCESS_CORE
product_converstion_ratio=[ratio],
cloudpak_id=[cloudpak_id],
cloudpak_name=[cloudpak_name],
cloudpak_metric=[cloudpak_metric],
[other parameters]} [METRIC_VALUE] # subproduct metric quantity
```

```yaml
apiVersion: marketplace.redhat.com/v1alpha2
kind: MeterDefinition
metadata:
  name: example-meterdefinition-2
  namespace: metering-example-operator
spec:
  resourceFilter:
    - type: operatorGroup # operatorGroup or namespace required
      useOperatorGroup: true
    - type: resourceKind # optional, would default to our default ones to look for
      kind: Service
    - type: ownerCRD
      apiVersion: partner.metering.com/v1alpha1
      kind: App
  aggregation: max
  period: 24h
  group: ${cloudpak_id}.partner.metering.com # ABC-DEF.partner.meter.com => reuslt in the reprot
  kind: product_license_usage
  metric: ${product_metric} # VIRTUAL_PROCESS_CORE or w/e other things we monitor here
  name: Product license usage data for ${product_metric} # optional
  groupBy: product_metric, cloudpak_id
  query: >-
    product_license_usage{}
---
apiVersion: marketplace.redhat.com/v1alpha2
kind: MeterDefinition
metadata:
  name: example-meterdefinition-2
  namespace: metering-example-operator
spec:
  resourceFilter:
    - type: operatorGroup # operatorGroup or namespace required
      useOperatorGroup: true
    - type: resourceKind # optional, would default to our default ones to look for
      kind: Service
    - type: ownerCRD
      apiVersion: partner.metering.com/v1alpha1
      kind: App
  aggregation: max
  period: 24h
  group: ${cloudpak_id}.partner.metering.com # ABC-DEF.partner.meter.com => reuslt in the reprot
  kind: product_license_usage_${product_id}
  metric: ${product_metric} # VIRTUAL_PROCESS_CORE or w/e other things we monitor here
  name: Product license usage data for ${product_metric} # optional
  groupBy: product_metric, product_id
  query: >-
    product_license_usage_details{}
```
