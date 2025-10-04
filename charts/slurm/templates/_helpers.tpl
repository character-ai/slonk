{{- define "slurm.gscNodepools" }}
output:
{{- $namespace := .namespace }}
{{- $template := .template }}
{{- range $name, $config := .configs }}
{{- range $i, $e := until ($config.numSlices | int) }}
  {{ $name }}-{{ $i }}:
    replicas: {{ $config.replicasPerSlice }}
    nodeSelector:
      node.kubernetes.io/instance-type: a3-highgpu-8g # This is showing the instance??
      cloud.google.com/gke-nodepool: {{ $config.nodepoolPrefix }}-{{ add $i 1 }}
    {{ $template | toYaml | indent 4 | trim }}
{{- end }}
{{- end }}
{{- end }}
