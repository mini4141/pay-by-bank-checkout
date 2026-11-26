resource "random_password" "db" {
  length  = 24
  special = false
}

resource "random_password" "webhook" {
  length  = 24
  special = false
}

resource "aws_db_subnet_group" "main" {
  name       = var.name
  subnet_ids = aws_subnet.public[*].id
}

resource "aws_db_instance" "main" {
  identifier              = var.name
  engine                  = "postgres"
  engine_version          = "17.12"
  instance_class          = "db.t4g.micro"
  allocated_storage       = 20
  storage_type            = "gp3"
  db_name                 = "pay"
  username                = "pay"
  password                = random_password.db.result
  db_subnet_group_name    = aws_db_subnet_group.main.name
  vpc_security_group_ids  = [aws_security_group.rds.id]
  publicly_accessible     = false
  multi_az                = false
  deletion_protection     = false
  skip_final_snapshot     = true
  backup_retention_period = 0
  apply_immediately       = true
}

resource "aws_ssm_parameter" "database_url" {
  name  = "/${var.name}/database-url"
  type  = "SecureString"
  value = "postgres://pay:${random_password.db.result}@${aws_db_instance.main.address}:5432/pay?sslmode=require"
}
