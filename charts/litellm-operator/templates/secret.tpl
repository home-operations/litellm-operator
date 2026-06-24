{{- if .Values.webhook.enabled }}
apiVersion: v1
kind: Secret
metadata:
  name: {{ include "litellm-operator.webhookCertName" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "litellm-operator.labels" . | nindent 4 }}
{{- end }}
