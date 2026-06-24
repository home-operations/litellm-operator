{{/*
Expand the name of the chart.
*/}}
{{- define "litellm-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "litellm-operator.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "litellm-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "litellm-operator.labels" -}}
helm.sh/chart: {{ include "litellm-operator.chart" . }}
{{ include "litellm-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "litellm-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "litellm-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "litellm-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "litellm-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the image name
*/}}
{{- define "litellm-operator.image" -}}
{{- if .Values.image.digest -}}
{{- printf "%s@%s" .Values.image.repository .Values.image.digest -}}
{{- else -}}
{{- printf "%s:%s" .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) -}}
{{- end -}}
{{- end }}

{{/*
Return the proper image pull policy
*/}}
{{- define "litellm-operator.imagePullPolicy" -}}
{{- .Values.image.pullPolicy | default "IfNotPresent" }}
{{- end }}

{{/*
Webhook service name
*/}}
{{- define "litellm-operator.webhookServiceName" -}}
{{- printf "%s-webhook" (include "litellm-operator.fullname" .) }}
{{- end }}

{{/*
Webhook serving-cert Secret name
*/}}
{{- define "litellm-operator.webhookCertName" -}}
{{- printf "%s-webhook-cert" (include "litellm-operator.fullname" .) }}
{{- end }}

{{/*
ValidatingWebhookConfiguration name
*/}}
{{- define "litellm-operator.webhookConfigName" -}}
{{- printf "%s-validating-webhook" (include "litellm-operator.fullname" .) }}
{{- end }}

{{/*
Metrics service name
*/}}
{{- define "litellm-operator.metricsServiceName" -}}
{{- printf "%s-metrics" (include "litellm-operator.fullname" .) }}
{{- end }}
