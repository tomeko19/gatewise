{{/*
Expand the name of the chart.
*/}}
{{- define "gatewise.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "gatewise.fullname" -}}
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
{{- define "gatewise.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "gatewise.labels" -}}
helm.sh/chart: {{ include "gatewise.chart" . }}
{{ include "gatewise.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "gatewise.selectorLabels" -}}
app.kubernetes.io/name: {{ include "gatewise.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Control Plane selector labels
*/}}
{{- define "gatewise.controlPlane.selectorLabels" -}}
{{ include "gatewise.selectorLabels" . }}
app.kubernetes.io/component: control-plane
{{- end }}

{{/*
Agent selector labels
*/}}
{{- define "gatewise.agent.selectorLabels" -}}
{{ include "gatewise.selectorLabels" . }}
app.kubernetes.io/component: agent
{{- end }}

{{/*
Dashboard selector labels
*/}}
{{- define "gatewise.dashboard.selectorLabels" -}}
{{ include "gatewise.selectorLabels" . }}
app.kubernetes.io/component: dashboard
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "gatewise.serviceAccountName" -}}
{{- if .Values.agent.serviceAccount.create }}
{{- default (printf "%s-agent" (include "gatewise.fullname" .)) .Values.agent.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.agent.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
PostgreSQL connection string
*/}}
{{- define "gatewise.postgresql.connectionString" -}}
{{- if .Values.postgresql.enabled }}
postgres://{{ .Values.postgresql.auth.username }}:{{ .Values.postgresql.auth.password }}@{{ include "gatewise.fullname" . }}-postgresql:5432/{{ .Values.postgresql.auth.database }}?sslmode=disable
{{- else }}
postgres://{{ .Values.postgresql.external.username }}:{{ .Values.postgresql.external.password }}@{{ .Values.postgresql.external.host }}:{{ .Values.postgresql.external.port }}/{{ .Values.postgresql.external.database }}?sslmode={{ .Values.postgresql.external.sslMode }}
{{- end }}
{{- end }}

{{/*
Kafka brokers
*/}}
{{- define "gatewise.kafka.brokers" -}}
{{- if .Values.kafka.enabled }}
{{- printf "%s-kafka:9092" (include "gatewise.fullname" .) }}
{{- else }}
{{- join "," .Values.kafka.external.brokers }}
{{- end }}
{{- end }}

{{/*
Kong admin URL
*/}}
{{- define "gatewise.kong.adminUrl" -}}
{{- if .Values.kong.enabled }}
{{- printf "http://%s-kong-admin:8001" (include "gatewise.fullname" .) }}
{{- else }}
{{- .Values.kong.external.adminUrl }}
{{- end }}
{{- end }}
