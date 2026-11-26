#!/usr/bin/env bash
# Tears down everything created by deploy.sh. Idempotent and safe to re-run.
# Anything that would block `terraform destroy` is removed first: ECS services scaled to 0,
# the S3 bucket and the ECR repositories emptied.
set -uo pipefail
cd "$(dirname "$0")/.."

export AWS_PROFILE=pay-by-bank-checkout-deployer AWS_REGION=ap-southeast-2 AWS_DEFAULT_REGION=ap-southeast-2
export MSYS_NO_PATHCONV=1

TF=(terraform -chdir=infra)
DEPLOYER_IP="$(curl -fsS https://checkip.amazonaws.com | tr -d '[:space:]' || echo 203.0.113.1)"

if "${TF[@]}" state list 2>/dev/null | grep -q .; then
  CLUSTER="$("${TF[@]}" output -raw cluster_name 2>/dev/null || echo pay-by-bank)"
  echo "==> scale ECS services to 0"
  for svc in api worker mockbank receiver checkout temporal; do
    aws ecs update-service --cluster "$CLUSTER" --service "$svc" --desired-count 0 >/dev/null 2>&1 || true
  done

  BUCKET="$("${TF[@]}" output -raw dashboard_bucket 2>/dev/null || true)"
  if [ -n "$BUCKET" ]; then
    echo "==> empty S3 bucket $BUCKET"
    aws s3 rm "s3://$BUCKET" --recursive --only-show-errors || true
  fi

  echo "==> empty ECR repositories"
  for svc in api worker mockbank receiver checkout temporal; do
    IDS="$(aws ecr list-images --repository-name "pay-by-bank/$svc" --query 'imageIds[*]' --output json 2>/dev/null || echo '[]')"
    if [ "$IDS" != "[]" ] && [ -n "$IDS" ]; then
      aws ecr batch-delete-image --repository-name "pay-by-bank/$svc" --image-ids "$IDS" >/dev/null 2>&1 || true
    fi
  done
else
  echo "no terraform state found; running destroy anyway (it will be a no-op)"
fi

echo "==> terraform destroy"
"${TF[@]}" init -input=false >/dev/null
"${TF[@]}" destroy -input=false -auto-approve -var "deployer_cidr=${DEPLOYER_IP}/32"
