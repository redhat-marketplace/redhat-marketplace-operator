kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: redhat-marketplace-operator
subjects:
- kind: ServiceAccount
  name: redhat-marketplace-operator
  namespace: {{ .NAMESPACE }}
roleRef:
  kind: ClusterRole
  name: redhat-marketplace-operator
  apiGroup: rbac.authorization.k8s.io
