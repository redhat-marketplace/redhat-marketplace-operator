# {{ print "GENERATED CODE - DO NOT EDIT OR COMMIT" }}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redhat-marketplace-operator
  namespace: redhat-marketplace-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      name: redhat-marketplace-operator
  template:
    metadata:
      labels:
        name: redhat-marketplace-operator
    spec:
      serviceAccountName: redhat-marketplace-operator
      containers:
        - name: redhat-marketplace-operator
          # Replace this with the built image name
          image: {{ .RELATED_IMAGE_MARKETPLACE_OPERATOR }}
          command:
          - redhat-marketplace-operator
          imagePullPolicy: {{ .PULL_POLICY }}
          env:
            - name: OPERATOR_NAME
              value: "redhat-marketplace-operator"
            - name: RELATED_IMAGE_MARKETPLACE_AGENT
              value: "{{ .RELATED_IMAGE_MARKETPLACE_AGENT }}"
            - name: RELATED_IMAGE_PROM_SERVER
              value: "prom/prometheus:v2.15.2"
            - name: RELATED_IMAGE_CONFIGMAP_RELOAD
              value: "jimmidyson/configmap-reload:v0.3.0"
            - name: WATCH_NAMESPACE
              value: "" # watch all namespaces
            - name: POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redhat-marketplace-razeedeploy-operator
  namespace: redhat-marketplace-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      name: redhat-marketplace-razeedeploy-operator
  template:
    metadata:
      labels:
        name: redhat-marketplace-razeedeploy-operator
    spec:
      serviceAccountName: redhat-marketplace-operator
      containers:
        - name: redhat-marketplace-operator
          # Replace this with the built image name
          image: {{ .RELATED_IMAGE_MARKETPLACE_OPERATOR }}
          command:
          - redhat-marketplace-razeedeploy-operator
          imagePullPolicy: {{ .PULL_POLICY }}
          env:
            - name: OPERATOR_NAME
              value: "redhat-marketplace-razeedeploy-operator"
            - name: RELATED_IMAGE_MARKETPLACE_AGENT
              value: "{{ .RELATED_IMAGE_MARKETPLACE_AGENT }}"
            - name: RELATED_IMAGE_PROM_SERVER
              value: "prom/prometheus:v2.15.2"
            - name: RELATED_IMAGE_CONFIGMAP_RELOAD
              value: "jimmidyson/configmap-reload:v0.3.0"
            - name: WATCH_NAMESPACE
              value: "" # watch all namespaces
            - name: POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redhat-marketplace-meterbase-operator
  namespace: redhat-marketplace-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      name: redhat-marketplace-meterbase-operator
  template:
    metadata:
      labels:
        name: redhat-marketplace-meterbase-operator
    spec:
      serviceAccountName: redhat-marketplace-operator
      containers:
        - name: redhat-marketplace-operator
          # Replace this with the built image name
          image: {{ .RELATED_IMAGE_MARKETPLACE_OPERATOR }}
          command:
          - redhat-marketplace-meterbase-operator
          imagePullPolicy: {{ .PULL_POLICY }}
          env:
            - name: OPERATOR_NAME
              value: "redhat-marketplace-meterbase-operator"
            - name: RELATED_IMAGE_MARKETPLACE_AGENT
              value: "{{ .RELATED_IMAGE_MARKETPLACE_AGENT }}"
            - name: RELATED_IMAGE_PROM_SERVER
              value: "prom/prometheus:v2.15.2"
            - name: RELATED_IMAGE_CONFIGMAP_RELOAD
              value: "jimmidyson/configmap-reload:v0.3.0"
            - name: WATCH_NAMESPACE
              value: "" # watch all namespaces
            - name: POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
