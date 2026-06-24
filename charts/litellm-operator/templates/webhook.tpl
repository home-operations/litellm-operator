{{- if .Values.webhook.enabled }}
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: {{ include "litellm-operator.webhookConfigName" . }}
  labels:
    {{- include "litellm-operator.labels" . | nindent 4 }}
webhooks:
  - name: vlitellmproxy.kb.io
    admissionReviewVersions:
      - v1
    clientConfig:
      service:
        name: {{ include "litellm-operator.webhookServiceName" . }}
        namespace: {{ .Release.Namespace }}
        path: /validate-litellm-home-operations-com-v1alpha1-litellmproxy
    failurePolicy: Fail
    sideEffects: None
    rules:
      - apiGroups:
          - litellm.home-operations.com
        apiVersions:
          - v1alpha1
        operations:
          - CREATE
          - UPDATE
        resources:
          - litellmproxies
  - name: vlitellmmodel.kb.io
    admissionReviewVersions:
      - v1
    clientConfig:
      service:
        name: {{ include "litellm-operator.webhookServiceName" . }}
        namespace: {{ .Release.Namespace }}
        path: /validate-litellm-home-operations-com-v1alpha1-litellmmodel
    failurePolicy: Fail
    sideEffects: None
    rules:
      - apiGroups:
          - litellm.home-operations.com
        apiVersions:
          - v1alpha1
        operations:
          - CREATE
          - UPDATE
        resources:
          - litellmmodels
---
apiVersion: v1
kind: Service
metadata:
  name: {{ include "litellm-operator.webhookServiceName" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "litellm-operator.labels" . | nindent 4 }}
spec:
  ports:
    - port: 443
      protocol: TCP
      targetPort: webhook
      name: webhook
  selector:
    {{- include "litellm-operator.selectorLabels" . | nindent 4 }}
{{- end }}
