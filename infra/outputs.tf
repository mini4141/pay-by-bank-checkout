output "alb_dns_name" {
  value = aws_lb.main.dns_name
}

output "checkout_url" {
  value = "http://${aws_lb.main.dns_name}"
}

output "api_url" {
  value = "http://${aws_lb.main.dns_name}/api"
}

output "receiver_url" {
  value = "http://${aws_lb.main.dns_name}/receiver"
}

output "temporal_ui_url" {
  value = "http://${aws_lb.main.dns_name}:8233"
}

output "dashboard_url" {
  value = "https://${aws_cloudfront_distribution.dashboard.domain_name}"
}

output "cloudfront_distribution_id" {
  value = aws_cloudfront_distribution.dashboard.id
}

output "dashboard_bucket" {
  value = aws_s3_bucket.dashboard.bucket
}

output "ecr_registry" {
  value = split("/", aws_ecr_repository.svc["api"].repository_url)[0]
}

output "ecr_repositories" {
  value = { for k, r in aws_ecr_repository.svc : k => r.repository_url }
}

output "cluster_name" {
  value = aws_ecs_cluster.main.name
}

output "migrate_task_definition" {
  value = aws_ecs_task_definition.svc["api"].family
}

output "subnet_ids" {
  value = aws_subnet.public[*].id
}

output "tasks_security_group_id" {
  value = aws_security_group.tasks.id
}

output "db_endpoint" {
  value = aws_db_instance.main.address
}

output "webhook_secret" {
  value     = random_password.webhook.result
  sensitive = true
}
