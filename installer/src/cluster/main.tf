# ECS Cluster for Installer Application
# Creates an ECS cluster with Fargate capacity provider

resource "aws_ecs_cluster" "main" {
  name = "installer-${var.nuon_id}"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }

  tags = {
    Name   = "installer-${var.nuon_id}"
    NuonID = var.nuon_id
  }
}

resource "aws_ecs_cluster_capacity_providers" "main" {
  cluster_name = aws_ecs_cluster.main.name

  capacity_providers = ["FARGATE", "FARGATE_SPOT"]

  default_capacity_provider_strategy {
    base              = 1
    weight            = 100
    capacity_provider = "FARGATE"
  }
}

# CloudWatch Log Group for ECS tasks
resource "aws_cloudwatch_log_group" "ecs" {
  name              = "/ecs/installer-${var.nuon_id}"
  retention_in_days = 30

  tags = {
    Name   = "installer-${var.nuon_id}"
    NuonID = var.nuon_id
  }
}
