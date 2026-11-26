#!/usr/bin/env bash
# Verifies nothing billable remains. Writes proof/phase3-destroy/verify-destroyed.txt and exits
# non-zero if any direct check still finds a resource (the tagging API lags, so its hits are
# informational; every billable type is confirmed with a direct describe/list call).
set -uo pipefail
cd "$(dirname "$0")/.."

export AWS_PROFILE=pay-by-bank-checkout-deployer AWS_REGION=ap-southeast-2 AWS_DEFAULT_REGION=ap-southeast-2
export MSYS_NO_PATHCONV=1
DEPLOYER_IP="$(curl -fsS https://checkip.amazonaws.com | tr -d '[:space:]' || echo none)"
redact() { sed -E "s/[0-9]{12}/<ACCOUNT_ID>/g; s/${DEPLOYER_IP//./\\.}/<DEPLOYER_IP>/g"; }

mkdir -p proof/phase3-destroy
OUT=proof/phase3-destroy/verify-destroyed.txt
LEFT=0

{
  echo "verify-destroyed: $(date -u +%Y-%m-%dT%H:%M:%SZ) region=$AWS_REGION"
  check() { # name, command output that must be empty
    local name="$1" found="$2"
    if [ -z "$found" ] || [ "$found" = "None" ]; then
      echo "OK    $name: none"
    else
      echo "LEFT  $name: $found"; LEFT=1
    fi
  }

  check "ECS cluster" "$(aws ecs describe-clusters --clusters pay-by-bank --query 'clusters[?status!=`INACTIVE`].clusterArn' --output text)"
  check "ALB" "$(aws elbv2 describe-load-balancers --query 'LoadBalancers[?starts_with(LoadBalancerName,`pay-by-bank`)].LoadBalancerArn' --output text 2>/dev/null)"
  check "target groups" "$(aws elbv2 describe-target-groups --query 'TargetGroups[?starts_with(TargetGroupName,`pay-by-bank`)].TargetGroupArn' --output text 2>/dev/null)"
  check "RDS instance" "$(aws rds describe-db-instances --query 'DBInstances[?starts_with(DBInstanceIdentifier,`pay-by-bank`)].DBInstanceIdentifier' --output text)"
  check "CloudFront distribution" "$(aws cloudfront list-distributions --query 'DistributionList.Items[?contains(Comment,`pay-by-bank`)].Id' --output text)"
  check "ECR repositories" "$(aws ecr describe-repositories --query 'repositories[?starts_with(repositoryName,`pay-by-bank/`)].repositoryName' --output text)"
  check "Cloud Map namespace" "$(aws servicediscovery list-namespaces --query 'Namespaces[?Name==`pay.local`].Id' --output text)"
  check "S3 dashboard bucket" "$(aws s3api list-buckets --query 'Buckets[?starts_with(Name,`pay-by-bank-dashboard`)].Name' --output text)"
  check "NAT gateways (tagged)" "$(aws ec2 describe-nat-gateways --filter Name=tag:Project,Values=pay-by-bank-checkout Name=state,Values=pending,available --query 'NatGateways[].NatGatewayId' --output text)"
  check "VPC (tagged)" "$(aws ec2 describe-vpcs --filters Name=tag:Project,Values=pay-by-bank-checkout --query 'Vpcs[].VpcId' --output text)"
  check "Elastic IPs (tagged)" "$(aws ec2 describe-addresses --filters Name=tag:Project,Values=pay-by-bank-checkout --query 'Addresses[].AllocationId' --output text)"
  check "CloudWatch log groups" "$(aws logs describe-log-groups --log-group-name-prefix /ecs/pay-by-bank --query 'logGroups[].logGroupName' --output text)"
  check "SSM parameters" "$(aws ssm describe-parameters --parameter-filters Key=Name,Option=BeginsWith,Values=/pay-by-bank --query 'Parameters[].Name' --output text)"

  echo
  echo "Tagging API (Project=pay-by-bank-checkout), informational - it can lag for a few minutes:"
  TAGGED="$(aws resourcegroupstaggingapi get-resources --tag-filters Key=Project,Values=pay-by-bank-checkout --query 'ResourceTagMappingList[].ResourceARN' --output text | tr '\t' '\n')"
  if [ -z "$TAGGED" ]; then echo "  no tagged resources"; else echo "$TAGGED" | sed 's/^/  /'; fi

  echo
  if [ "$LEFT" -eq 0 ]; then echo "RESULT: nothing billable remains"; else echo "RESULT: resources remain"; fi
  exit "$LEFT"
} 2>&1 | redact | tee "$OUT"
exit "${PIPESTATUS[0]}"
