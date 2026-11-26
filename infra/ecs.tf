resource "aws_ecs_cluster" "main" {
  name = var.name
}

resource "aws_cloudwatch_log_group" "svc" {
  for_each          = toset(local.repos)
  name              = "/ecs/${var.name}/${each.key}"
  retention_in_days = 1
}

# ---- IAM: task execution role (pull from ECR, write logs, read the DB URL parameter) ----

data "aws_iam_policy_document" "ecs_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "exec" {
  name               = "${var.name}-task-exec"
  assume_role_policy = data.aws_iam_policy_document.ecs_assume.json
}

resource "aws_iam_role_policy_attachment" "exec" {
  role       = aws_iam_role.exec.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

data "aws_iam_policy_document" "exec_ssm" {
  statement {
    actions   = ["ssm:GetParameters", "ssm:GetParameter"]
    resources = [aws_ssm_parameter.database_url.arn]
  }
  statement {
    actions   = ["kms:Decrypt"]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "exec_ssm" {
  name   = "read-database-url"
  role   = aws_iam_role.exec.id
  policy = data.aws_iam_policy_document.exec_ssm.json
}

# ---- services ----

locals {
  alb_origin = "http://${aws_lb.main.dns_name}"
  db_secret  = [{ name = "DATABASE_URL", valueFrom = aws_ssm_parameter.database_url.arn }]

  services = {
    temporal = {
      cpu     = 512, memory = 1024, ports = [7233, 8233], lb_port = 8233, discovery = true
      command = ["server", "start-dev", "--ip", "0.0.0.0", "--ui-ip", "0.0.0.0", "--port", "7233", "--ui-port", "8233", "--log-level", "warn"]
      env     = {}
      secrets = []
    }
    mockbank = {
      cpu     = 256, memory = 512, ports = [8081], lb_port = null, discovery = true, command = null
      env     = { MOCKBANK_FAIL_RATE = "0.5", MOCKBANK_ADDR = ":8081" }
      secrets = []
    }
    receiver = {
      cpu     = 256, memory = 512, ports = [8082], lb_port = 8082, discovery = true, command = null
      env     = { WEBHOOK_SECRET = random_password.webhook.result, RECEIVER_ADDR = ":8082" }
      secrets = []
    }
    api = {
      cpu = 256, memory = 512, ports = [8080], lb_port = 8080, discovery = true, command = null
      env = {
        TEMPORAL_ADDRESS     = "temporal.pay.local:7233"
        CHECKOUT_BASE_URL    = local.alb_origin
        CORS_ALLOWED_ORIGINS = local.alb_origin
        API_ADDR             = ":8080"
      }
      secrets = local.db_secret
    }
    worker = {
      cpu = 256, memory = 512, ports = [8083], lb_port = null, discovery = false, command = null
      env = {
        TEMPORAL_ADDRESS = "temporal.pay.local:7233"
        MOCKBANK_URL     = "http://mockbank.pay.local:8081"
        RISK_LIMIT_MINOR = "1000000"
      }
      secrets = local.db_secret
    }
    checkout = {
      cpu     = 256, memory = 512, ports = [3000], lb_port = 3000, discovery = false, command = null
      env     = { PUBLIC_API_URL = "/api", ORIGIN = local.alb_origin, PORT = "3000" }
      secrets = []
    }
  }
}

resource "aws_ecs_task_definition" "svc" {
  for_each                 = local.services
  family                   = "${var.name}-${each.key}"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = each.value.cpu
  memory                   = each.value.memory
  execution_role_arn       = aws_iam_role.exec.arn

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "X86_64"
  }

  container_definitions = jsonencode([{
    name         = each.key
    image        = "${aws_ecr_repository.svc[each.key].repository_url}:latest"
    essential    = true
    command      = each.value.command
    portMappings = [for p in each.value.ports : { containerPort = p, protocol = "tcp" }]
    environment  = [for k, v in each.value.env : { name = k, value = v }]
    secrets      = each.value.secrets
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = aws_cloudwatch_log_group.svc[each.key].name
        awslogs-region        = var.region
        awslogs-stream-prefix = each.key
      }
    }
  }])
}

resource "aws_service_discovery_service" "svc" {
  for_each = { for k, v in local.services : k => v if v.discovery }
  name     = each.key
  dns_config {
    namespace_id = aws_service_discovery_private_dns_namespace.main.id
    dns_records {
      type = "A"
      ttl  = 10
    }
    routing_policy = "MULTIVALUE"
  }
  force_destroy = true
}

resource "aws_ecs_service" "svc" {
  for_each                           = local.services
  name                               = each.key
  cluster                            = aws_ecs_cluster.main.id
  task_definition                    = aws_ecs_task_definition.svc[each.key].arn
  desired_count                      = var.desired_count
  launch_type                        = "FARGATE"
  health_check_grace_period_seconds  = each.value.lb_port != null ? 60 : null
  deployment_minimum_healthy_percent = 0
  deployment_maximum_percent         = 100
  force_delete                       = true

  network_configuration {
    subnets          = aws_subnet.public[*].id
    security_groups  = [aws_security_group.tasks.id]
    assign_public_ip = true
  }

  dynamic "load_balancer" {
    for_each = each.value.lb_port != null ? [1] : []
    content {
      target_group_arn = aws_lb_target_group.svc[each.key].arn
      container_name   = each.key
      container_port   = each.value.lb_port
    }
  }

  dynamic "service_registries" {
    for_each = each.value.discovery ? [1] : []
    content {
      registry_arn = aws_service_discovery_service.svc[each.key].arn
    }
  }

  depends_on = [aws_lb_listener.http, aws_lb_listener.temporal_ui, aws_lb_listener_rule.path, aws_lb_listener_rule.healthz]
}
