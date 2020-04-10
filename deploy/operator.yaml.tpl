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
          imagePullPolicy: Always
          env:
            - name: OPERATOR_NAME
              value: "redhat-marketplace-operator"
            - name: RELATED_IMAGE_MARKETPLACE_AGENT
              value: "{{ .RELATED_IMAGE_MARKETPLACE_AGENT }}"
            - name: RELATED_IMAGE_PROM_SERVER
              value: "prom/prometheus:v2.15.2"
            - name: RELATED_IMAGE_CONFIGMAP_RELOAD
              value: "jimmidyson/configmap-reload:v0.3.0"
            - name: RELATED_IMAGE_RAZEE_JOB
              value: "quay.io/razee/razeedeploy-delta:1.1.0"
            - name: WATCH_NAMESPACE
              value: "" # watch all namespaces
            - name: POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
