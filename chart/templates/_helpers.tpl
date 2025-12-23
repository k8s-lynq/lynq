{{/*
Expand the name of the chart.
*/}}
{{- define "lynq-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "lynq-operator.fullname" -}}
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
{{- define "lynq-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "lynq-operator.labels" -}}
helm.sh/chart: {{ include "lynq-operator.chart" . }}
{{ include "lynq-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "lynq-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "lynq-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
control-plane: controller-manager
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "lynq-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "lynq-operator.fullname" . | printf "%s-controller-manager") .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the namespace name
*/}}
{{- define "lynq-operator.namespace" -}}
{{- if .Values.namespace.name }}
{{- .Values.namespace.name }}
{{- else }}
{{- .Release.Namespace }}
{{- end }}
{{- end }}

{{/*
Create the image name
*/}}
{{- define "lynq-operator.image" -}}
{{- $registry := .Values.image.registry }}
{{- $repository := .Values.image.repository }}
{{- $tag := .Values.image.tag | default .Chart.AppVersion }}
{{- printf "%s/%s:%s" $registry $repository $tag }}
{{- end }}

{{/*
Common annotations
*/}}
{{- define "lynq-operator.annotations" -}}
{{- with .Values.commonAnnotations }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Manager container name
*/}}
{{- define "lynq-operator.managerContainerName" -}}
manager
{{- end }}

{{/*
Metrics service name
*/}}
{{- define "lynq-operator.metricsServiceName" -}}
{{- include "lynq-operator.fullname" . }}-metrics-service
{{- end }}

{{/*
Webhook service name
*/}}
{{- define "lynq-operator.webhookServiceName" -}}
{{- include "lynq-operator.fullname" . }}-webhook-service
{{- end }}

{{/*
Webhook certificate name
*/}}
{{- define "lynq-operator.webhookCertificateName" -}}
{{- include "lynq-operator.fullname" . }}-serving-cert
{{- end }}

{{/*
Leader election role name
*/}}
{{- define "lynq-operator.leaderElectionRoleName" -}}
{{- include "lynq-operator.fullname" . }}-leader-election-role
{{- end }}

{{/*
Manager role name
*/}}
{{- define "lynq-operator.managerRoleName" -}}
{{- include "lynq-operator.fullname" . }}-manager-role
{{- end }}

{{/*
Metrics reader role name
*/}}
{{- define "lynq-operator.metricsReaderRoleName" -}}
{{- include "lynq-operator.fullname" . }}-metrics-reader
{{- end }}

{{/*
Proxy role name
*/}}
{{- define "lynq-operator.proxyRoleName" -}}
{{- include "lynq-operator.fullname" . }}-proxy-role
{{- end }}

{{/*
Manager role binding name
*/}}
{{- define "lynq-operator.managerRoleBindingName" -}}
{{- include "lynq-operator.fullname" . }}-manager-rolebinding
{{- end }}

{{/*
Leader election role binding name
*/}}
{{- define "lynq-operator.leaderElectionRoleBindingName" -}}
{{- include "lynq-operator.fullname" . }}-leader-election-rolebinding
{{- end }}

{{/*
Proxy role binding name
*/}}
{{- define "lynq-operator.proxyRoleBindingName" -}}
{{- include "lynq-operator.fullname" . }}-proxy-rolebinding
{{- end }}

{{/*
ValidatingWebhookConfiguration name
*/}}
{{- define "lynq-operator.validatingWebhookName" -}}
{{- include "lynq-operator.fullname" . }}-validating-webhook-configuration
{{- end }}

{{/*
MutatingWebhookConfiguration name
*/}}
{{- define "lynq-operator.mutatingWebhookName" -}}
{{- include "lynq-operator.fullname" . }}-mutating-webhook-configuration
{{- end }}

{{/*
ServiceMonitor name
*/}}
{{- define "lynq-operator.serviceMonitorName" -}}
{{- include "lynq-operator.fullname" . }}-metrics-monitor
{{- end }}

{{/*
Validate webhook configuration
*/}}
{{- define "lynq-operator.validateWebhook" -}}
{{- if and .Values.webhook.enabled (not .Values.certManager.enabled) }}
{{- fail "webhook.enabled requires certManager.enabled to be true" }}
{{- end }}
{{- end }}

{{/*
Dashboard service account name
*/}}
{{- define "lynq-operator.dashboardServiceAccountName" -}}
{{- if .Values.dashboard.serviceAccount.create }}
{{- default (printf "%s-dashboard" (include "lynq-operator.fullname" .)) .Values.dashboard.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.dashboard.serviceAccount.name }}
{{- end }}
{{- end }}
