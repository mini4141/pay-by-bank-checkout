resource "aws_lb" "main" {
  name               = var.name
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = aws_subnet.public[*].id
  idle_timeout       = 60
}

locals {
  # Target groups; the ones with a path get a listener rule on :80, checkout is the default.
  targets = {
    checkout = { port = 3000, health = "/", priority = 0, path = null }
    api      = { port = 8080, health = "/healthz", priority = 10, path = "/api/*" }
    receiver = { port = 8082, health = "/healthz", priority = 20, path = "/receiver/*" }
    temporal = { port = 8233, health = "/", priority = 0, path = null }
  }
}

resource "aws_lb_target_group" "svc" {
  for_each             = local.targets
  name                 = "${var.name}-${each.key}"
  port                 = each.value.port
  protocol             = "HTTP"
  target_type          = "ip"
  vpc_id               = aws_vpc.main.id
  deregistration_delay = 5

  health_check {
    path                = each.value.health
    matcher             = "200-399"
    interval            = 10
    healthy_threshold   = 2
    unhealthy_threshold = 5
  }
}

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.main.arn
  port              = 80
  protocol          = "HTTP"
  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.svc["checkout"].arn
  }
}

resource "aws_lb_listener_rule" "path" {
  for_each     = { for k, v in local.targets : k => v if v.path != null }
  listener_arn = aws_lb_listener.http.arn
  priority     = each.value.priority
  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.svc[each.key].arn
  }
  condition {
    path_pattern { values = [each.value.path] }
  }
}

# The API health endpoint is also reachable through the ALB for smoke tests.
resource "aws_lb_listener_rule" "healthz" {
  listener_arn = aws_lb_listener.http.arn
  priority     = 5
  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.svc["api"].arn
  }
  condition {
    path_pattern { values = ["/healthz"] }
  }
}

resource "aws_lb_listener" "temporal_ui" {
  load_balancer_arn = aws_lb.main.arn
  port              = 8233
  protocol          = "HTTP"
  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.svc["temporal"].arn
  }
}
