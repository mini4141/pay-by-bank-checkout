#!/usr/bin/env bash
# Deploys the whole stack to AWS from this PC (profile pay-by-bank-checkout-deployer, ap-southeast-2):
# terraform (infra, services at 0) -> push images -> upload dashboard -> migrate (one-off task)
# -> terraform (services at 1) -> seed merchant -> cloud e2e proof (unless SKIP_PROOF=1).
set -euo pipefail
cd "$(dirname "$0")/.."

export AWS_PROFILE=pay-by-bank-checkout-deployer AWS_REGION=ap-southeast-2 AWS_DEFAULT_REGION=ap-southeast-2
export MSYS_NO_PATHCONV=1 # Git Bash must not rewrite /ecs/... style arguments

aws sts get-caller-identity >/dev/null || { echo "AWS profile $AWS_PROFILE is not usable" >&2; exit 1; }
DEPLOYER_IP="$(curl -fsS https://checkip.amazonaws.com | tr -d '[:space:]')"
TF=(terraform -chdir=infra)
TFVARS=(-var "deployer_cidr=${DEPLOYER_IP}/32")
out() { "${TF[@]}" output -raw "$1"; }

echo "==> terraform apply (infrastructure, services scaled to 0)"
"${TF[@]}" init -input=false
"${TF[@]}" apply -input=false -auto-approve "${TFVARS[@]}" -var desired_count=0

REGISTRY="$(out ecr_registry)"
CLUSTER="$(out cluster_name)"
BUCKET="$(out dashboard_bucket)"
CF_ID="$(out cloudfront_distribution_id)"
ALB="$(out checkout_url)"

echo "==> build and push images"
aws ecr get-login-password | docker login --username AWS --password-stdin "$REGISTRY" >/dev/null
for svc in api worker mockbank receiver; do
  docker build -q -f services/Dockerfile --build-arg SERVICE="$svc" -t "$REGISTRY/pay-by-bank/$svc:latest" .
  docker push -q "$REGISTRY/pay-by-bank/$svc:latest" >/dev/null
done
docker build -q -f web/checkout/Dockerfile -t "$REGISTRY/pay-by-bank/checkout:latest" .
docker push -q "$REGISTRY/pay-by-bank/checkout:latest" >/dev/null
# Mirror the Temporal image into ECR so Fargate does not depend on Docker Hub rate limits.
docker pull -q temporalio/temporal:1.10.1 >/dev/null
docker tag temporalio/temporal:1.10.1 "$REGISTRY/pay-by-bank/temporal:latest"
docker push -q "$REGISTRY/pay-by-bank/temporal:latest" >/dev/null

echo "==> upload merchant dashboard to S3"
VITE_API_URL=/api pnpm --filter @pay/dashboard build
aws s3 sync web/dashboard/dist "s3://$BUCKET" --delete --only-show-errors
aws cloudfront create-invalidation --distribution-id "$CF_ID" --paths '/*' >/dev/null

echo "==> run migrations (one-off ECS task)"
SUBNETS="$("${TF[@]}" output -json subnet_ids | tr -d '[]" \r\n')"
SG="$(out tasks_security_group_id)"
TASK_ARN="$(aws ecs run-task --cluster "$CLUSTER" --launch-type FARGATE --task-definition "$(out migrate_task_definition)" \
  --network-configuration "awsvpcConfiguration={subnets=[$SUBNETS],securityGroups=[$SG],assignPublicIp=ENABLED}" \
  --overrides '{"containerOverrides":[{"name":"api","command":["migrate"]}]}' \
  --query 'tasks[0].taskArn' --output text)"
aws ecs wait tasks-stopped --cluster "$CLUSTER" --tasks "$TASK_ARN"
EXIT_CODE="$(aws ecs describe-tasks --cluster "$CLUSTER" --tasks "$TASK_ARN" --query 'tasks[0].containers[0].exitCode' --output text)"
[ "$EXIT_CODE" = "0" ] || { echo "migration task failed (exit $EXIT_CODE)" >&2; exit 1; }
echo "migrations applied"

echo "==> start services"
"${TF[@]}" apply -input=false -auto-approve "${TFVARS[@]}" -var desired_count=1
aws ecs wait services-stable --cluster "$CLUSTER" --services api worker mockbank receiver checkout temporal

echo "==> wait for the API behind the ALB"
for _ in $(seq 1 60); do
  curl -fsS "$ALB/healthz" >/dev/null 2>&1 && break
  sleep 5
done
curl -fsS "$ALB/healthz" >/dev/null

echo "==> seed demo merchant"
API_URL="$ALB/api" SEED_FILE=.local/seed-cloud.json \
  DEMO_WEBHOOK_URL="http://receiver.pay.local:8082/receiver/webhook" \
  DEMO_WEBHOOK_SECRET="$(out webhook_secret)" ./scripts/seed.sh

cat <<MSG

Deployed.
  Checkout / ALB   $ALB
  Dashboard        $(out dashboard_url)
  Temporal UI      $(out temporal_ui_url)   (only reachable from this PC's IP)
MSG

if [ -z "${SKIP_PROOF:-}" ]; then
  ./scripts/capture-cloud-proof.sh
fi
