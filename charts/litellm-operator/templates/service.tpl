---
apiVersion: v1
kind: Service
metadata:
  name: {{ include "litellm-operator.metricsServiceName" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "litellm-operator.labels" . | nindent 4 }}
  {{- with .Values.controller.metrics.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  type: ClusterIP
  ports:
    - port: {{ .Values.controller.metrics.port }}
      targetPort: metrics
      protocol: TCP
      name: metrics
  selector:
    {{- include "litellm-operator.selectorLabels" . | nindent 4 }}
