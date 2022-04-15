{{/* vim: set filetype=mustache: */}}
{{/*

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "cloudshell.controllerManager.fullname" -}}
{{- printf "%s-%s-%s" (include "common.names.fullname" .) "controller" "manager" | trunc 63 | trimSuffix "-" -}}
{{- end }}

{{/*
Return the proper image name
*/}}
{{- define "cloudshell.controllerManager.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper image Registry Secret Names
*/}}
{{- define "cloudshell.controllerManager.imagePullSecrets" -}}
{{ include "common.images.pullSecrets" (dict "images" (list .Values.image) "global" .Values.global) }}
{{- end -}}
