{{- define "autoscale-storage.requiredConfig" -}}
{{- required (printf "config value %s is required; pass it with --set-file" .name) .value -}}
{{- end -}}
