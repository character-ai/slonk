#!/bin/sh
cluster=${1:-tpu1}
shift
cat ../../cluster-addons/slurm/$cluster.yaml \
    | yq -r '.spec.template.spec.source.plugin.parameters[] | select(.name == "values-override") | .string' \
    | sed "s/{{name}}/$cluster/" \
    | helm template . -f values.yaml -f - $@
