{{- define "goshopx.name" -}}
goshopx
{{- end }}
{{- define "goshopx.fullname" -}}
{{- .service | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- define "goshopx.labels" -}}
app.kubernetes.io/name: {{ include "goshopx.name" .root }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/component: {{ .service }}
app.kubernetes.io/managed-by: Helm
{{- end }}
