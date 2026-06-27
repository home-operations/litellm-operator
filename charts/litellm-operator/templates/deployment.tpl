---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "litellm-operator.fullname" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "litellm-operator.labels" . | nindent 4 }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      {{- include "litellm-operator.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      {{- with .Values.podAnnotations }}
      annotations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      labels:
        {{- include "litellm-operator.labels" . | nindent 8 }}
        {{- with .Values.podLabels }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
    spec:
      enableServiceLinks: false
      {{- with .Values.imagePullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      serviceAccountName: {{ include "litellm-operator.serviceAccountName" . }}
      {{- with .Values.priorityClassName }}
      priorityClassName: {{ . }}
      {{- end }}
      securityContext:
        {{- toYaml .Values.podSecurityContext | nindent 8 }}
      containers:
        - name: manager
          securityContext:
            {{- toYaml .Values.securityContext | nindent 12 }}
          image: {{ include "litellm-operator.image" . }}
          imagePullPolicy: {{ include "litellm-operator.imagePullPolicy" . }}
          command:
            - /manager
          args:
            - --log-level={{ .Values.controller.logLevel }}
            - --leader-elect={{ .Values.controller.leaderElection.enabled }}
            - --health-probe-bind-address=:{{ .Values.controller.health.port }}
            {{- if .Values.controller.metrics.enabled }}
            - --metrics-bind-address=:{{ .Values.controller.metrics.port }}
            - --metrics-secure={{ .Values.controller.metrics.secure }}
            {{- else }}
            - --metrics-bind-address=0
            {{- end }}
            {{- if .Values.webhook.enabled }}
            - --webhook-config-name={{ include "litellm-operator.webhookConfigName" . }}
            - --webhook-service-name={{ include "litellm-operator.webhookServiceName" . }}
            - --webhook-secret-name={{ include "litellm-operator.webhookCertName" . }}
            {{- end }}
          env:
            - name: CONTROLLER_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            {{- if .Values.llmkube.autoRegister }}
            - name: ENABLE_LLMKUBE_AUTOREGISTER
              value: "true"
            {{- end }}
            {{- with .Values.env }}
            {{- toYaml . | nindent 12 }}
            {{- end }}
          ports:
            - name: health
              containerPort: {{ .Values.controller.health.port }}
              protocol: TCP
            {{- if .Values.controller.metrics.enabled }}
            - name: metrics
              containerPort: {{ .Values.controller.metrics.port }}
              protocol: TCP
            {{- end }}
            {{- if .Values.webhook.enabled }}
            - name: webhook
              containerPort: {{ .Values.webhook.port }}
              protocol: TCP
            {{- end }}
          livenessProbe:
            {{- toYaml .Values.livenessProbe | nindent 12 }}
          readinessProbe:
            {{- toYaml .Values.readinessProbe | nindent 12 }}
          resources:
            {{- toYaml .Values.resources | nindent 12 }}
          {{- if .Values.webhook.enabled }}
          volumeMounts:
            - name: webhook-cert
              mountPath: /tmp/k8s-webhook-server/serving-certs
              readOnly: true
          {{- end }}
      {{- if .Values.webhook.enabled }}
      volumes:
        - name: webhook-cert
          secret:
            secretName: {{ include "litellm-operator.webhookCertName" . }}
            defaultMode: 420
            optional: true
      {{- end }}
      {{- with .Values.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.affinity }}
      affinity:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.tolerations }}
      tolerations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
