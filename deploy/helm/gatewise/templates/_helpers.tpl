{{- define "gatewise.name" -}}
gatewise
{{- end -}}

{{- define "gatewise.fullname" -}}
{{- printf "%s" (include "gatewise.name" .) -}}
{{- end -}}
