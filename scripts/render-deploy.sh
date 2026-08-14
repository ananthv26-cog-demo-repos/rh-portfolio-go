#!/usr/bin/env bash
set -euo pipefail

road_root="${PAVED_ROAD_ROOT:-/home/ubuntu/repos/rh-paved-road}"
service_name="${SERVICE_NAME:-rh-portfolio-go}"
port="${PORT:-8081}"
template_root="$road_root/templates/go-service"

if [[ ! -f "$template_root/Dockerfile.tmpl" || ! -f "$template_root/deploy.yaml.tmpl" ]]; then
  echo "paved-road go-service templates not found under $template_root" >&2
  exit 1
fi

mkdir -p deploy
sed -e "s/{{SERVICE_NAME}}/$service_name/g" -e "s/{{PORT}}/$port/g" \
  "$template_root/Dockerfile.tmpl" > Dockerfile
sed -e "s/{{SERVICE_NAME}}/$service_name/g" -e "s/{{PORT}}/$port/g" \
  "$template_root/deploy.yaml.tmpl" > deploy/deploy.yaml
