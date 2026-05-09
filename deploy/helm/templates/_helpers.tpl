{{- define "compra-back.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- define "compra-back.fullname" -}}
{{- printf "%s" (include "compra-back.name" .) | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- define "compra-back.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
app.kubernetes.io/name: {{ include "compra-back.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}
{{- define "compra-back.selectorLabels" -}}
app.kubernetes.io/name: {{ include "compra-back.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
