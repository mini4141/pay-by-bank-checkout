#!/usr/bin/env bash
# Runs the Playwright demo story against the cloud endpoints and collects AWS-side proof
# into proof/phase2-cloud/. Called by deploy.sh, or run on its own while the stack is up.
set -euo pipefail
cd "$(dirname "$0")/.."

export AWS_PROFILE=pay-by-bank-checkout-deployer AWS_REGION=ap-southeast-2 AWS_DEFAULT_REGION=ap-southeast-2
export MSYS_NO_PATHCONV=1
TF=(terraform -chdir=infra)
out() { "${TF[@]}" output -raw "$1"; }

DEPLOYER_IP="$(curl -fsS https://checkip.amazonaws.com | tr -d '[:space:]')"
redact() { sed -E "s/[0-9]{12}/<ACCOUNT_ID>/g; s/(AKIA|ASIA)[A-Z0-9]{16}/<ACCESS_KEY>/g; s/${DEPLOYER_IP//./\\.}/<DEPLOYER_IP>/g"; }

P=proof/phase2-cloud
rm -rf "$P"; mkdir -p "$P/screenshots" "$P/logs" "$P/tests"
ABS="$(pwd -W 2>/dev/null || pwd)/$P"
CLUSTER="$(out cluster_name)"
SERVICES=(api worker mockbank receiver checkout temporal)

echo "==> cloud smoke + Playwright story"
# The story passes with 50% mock bank failures; whether a retry shows in the Temporal UI is random in the cloud.
DASHBOARD_URL="$(out dashboard_url)"
API_URL="$(out api_url)"
RECEIVER_URL="$(out receiver_url)"
TEMPORAL_UI_URL="$(out temporal_ui_url)"
export DASHBOARD_URL API_URL RECEIVER_URL TEMPORAL_UI_URL
set +e
(
  cd e2e
  CLOUD=1 \
  SEED_FILE=../.local/seed-cloud.json \
  PROOF_DIR="$ABS" PW_REPORT_DIR="$ABS/tests/playwright-cloud" \
  pnpm exec playwright test
) 2>&1 | redact | tee "$P/logs/playwright-cloud.log"
STATUS=${PIPESTATUS[0]}
set -e

# Names required by SPECS §10, copied from the story's screenshots.
cp "$P/screenshots/cloud-04-checkout-timeline-settled.png" "$P/screenshots/01-cloud-checkout-settled.png" || true
cp "$P/screenshots/cloud-05-dashboard-live-payment.png" "$P/screenshots/02-cloud-dashboard.png" || true
cp "$P/screenshots/cloud-07-temporal-ui-history-retries.png" "$P/screenshots/03-cloud-temporal-ui.png" || true

# The cloud mock bank fails 50% of debits, so some payment from this run has a retried debit: show it.
(
  cd e2e
  node scripts/retry-shot.mjs "$API_URL" "$(sed -n 's/.*"api_key": *"\([^"]*\)".*/\1/p' ../.local/seed-cloud.json)" \
    "$TEMPORAL_UI_URL" "$ABS/screenshots/05-cloud-temporal-retried-debit.png"
) 2>&1 | redact | tee -a "$P/logs/playwright-cloud.log" || true

echo "==> AWS-side evidence"
aws ecs describe-services --cluster "$CLUSTER" --services "${SERVICES[@]}" \
  --query 'services[].{service:serviceName,status:status,desired:desiredCount,running:runningCount,launchType:launchType,taskDefinition:taskDefinition}' \
  --output json | redact > "$P/logs/ecs-services.json"
MSYS_NO_PATHCONV=1 aws logs tail /ecs/pay-by-bank/worker --since 1h --format short | redact > "$P/logs/cloudwatch-worker.log"
MSYS_NO_PATHCONV=1 aws logs tail /ecs/pay-by-bank/mockbank --since 1h --format short | grep -E 'injected_failure|replayed|"ok"' | redact > "$P/logs/cloudwatch-mockbank.log" || true

{
  echo "# Deployed AWS resources (terraform outputs and tag query)"
  "${TF[@]}" output | grep -v webhook_secret
  echo
  echo "# Resources tagged Project=pay-by-bank-checkout"
  aws resourcegroupstaggingapi get-resources --tag-filters Key=Project,Values=pay-by-bank-checkout \
    --query 'ResourceTagMappingList[].ResourceARN' --output text | tr '\t' '\n' | sort
  echo
  echo "# RDS"
  aws rds describe-db-instances --db-instance-identifier pay-by-bank \
    --query 'DBInstances[].{id:DBInstanceIdentifier,class:DBInstanceClass,engine:Engine,version:EngineVersion,public:PubliclyAccessible,multiAZ:MultiAZ,status:DBInstanceStatus}' --output table
  echo "# Dashboard hosting"
  echo "(CloudFront is off: the AWS account needs Support verification. Dashboard is served from S3 website hosting, see ADR 0002.)"
  aws s3api get-bucket-website --bucket "$(out dashboard_bucket)" --output table
} 2>&1 | redact > "$P/resources.txt"

# A metrics graph rendered from the CLI (no console login): ALB request count.
ALB_ARN="$(aws elbv2 describe-load-balancers --names pay-by-bank --query 'LoadBalancers[0].LoadBalancerArn' --output text)"
ALB_DIM="${ALB_ARN#*:loadbalancer/}"
aws cloudwatch get-metric-widget-image --output text --query MetricWidgetImage --metric-widget "{
  \"width\":800,\"height\":300,\"title\":\"ALB request count\",\"start\":\"-PT1H\",\"end\":\"P0D\",\"period\":60,\"stat\":\"Sum\",
  \"metrics\":[[\"AWS/ApplicationELB\",\"RequestCount\",\"LoadBalancer\",\"$ALB_DIM\"]]}" \
  | tr -d '\r\n' | base64 -d > "$P/screenshots/04-cloud-alb-request-count.png" || echo "metric widget skipped"

# Test report copy for the index
[ -d "$P/tests/playwright-cloud" ] || true
echo "cloud proof captured in $P (playwright exit status: $STATUS)"
exit "$STATUS"
