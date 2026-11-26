locals {
  repos = ["api", "worker", "mockbank", "receiver", "checkout", "temporal"]
}

resource "aws_ecr_repository" "svc" {
  for_each             = toset(local.repos)
  name                 = "${var.name}/${each.key}"
  force_delete         = true
  image_tag_mutability = "MUTABLE"
}
