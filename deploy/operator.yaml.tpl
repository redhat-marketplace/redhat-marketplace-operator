apiVersion: apps/v1
kind: Deployment
metadata:
  name: marketplace-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      name: marketplace-operator
  template:
    metadata:
      labels:
        name: marketplace-operator
    spec:
      serviceAccountName: marketplace-operator
      containers:
        - name: marketplace-operator
          # Replace this with the built image name
          image: {{ .RELATED_IMAGE_MARKETPLACE_OPERATOR }}
          command:
          - marketplace-operator
          imagePullPolicy: Always
          env:
            - name: OPERATOR_NAME
              value: "marketplace-operator"
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
