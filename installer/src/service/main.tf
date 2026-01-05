# ECS Service Module - Single Service with Path-Based Routing
# Creates a single ECS Fargate service on port 8080 serving both vendor and customer interfaces
# Registers with both ALB target groups (each ALB routes to the same port via different domains)

data "aws_subnets" "private" {
  filter {
    name   = "vpc-id"
    values = [var.vpc_id]
  }

  filter {
    name   = "tag:Name"
    values = ["*private*"]
  }
}

data "aws_vpc" "main" {
  id = var.vpc_id
}

# IAM Role for ECS Task Execution
resource "aws_iam_role" "execution" {
  name = "installer-execution-${var.nuon_id}"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ecs-tasks.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Name   = "installer-execution-${var.nuon_id}"
    NuonID = var.nuon_id
  }
}

resource "aws_iam_role_policy_attachment" "execution" {
  role       = aws_iam_role.execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

# IAM Role for ECS Task (application permissions)
resource "aws_iam_role" "task" {
  name = "installer-task-${var.nuon_id}"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ecs-tasks.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Name   = "installer-task-${var.nuon_id}"
    NuonID = var.nuon_id
  }
}

# Security group for ECS tasks - allows traffic from BOTH ALBs on single port
resource "aws_security_group" "ecs" {
  name        = "installer-ecs-${var.nuon_id}"
  description = "Security group for installer ECS tasks"
  vpc_id      = var.vpc_id

  # Allow traffic from both ALBs on port 8080 (single port architecture)
  ingress {
    description     = "Allow traffic from ALBs"
    from_port       = 8080
    to_port         = 8080
    protocol        = "tcp"
    security_groups = [var.vendor_security_group_id, var.customer_security_group_id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name   = "installer-ecs-${var.nuon_id}"
    NuonID = var.nuon_id
  }
}

# CloudWatch Log Group for this service
resource "aws_cloudwatch_log_group" "service" {
  name              = "/ecs/installer-${var.nuon_id}"
  retention_in_days = 30

  tags = {
    Name   = "installer-${var.nuon_id}"
    NuonID = var.nuon_id
  }
}

# ECS Task Definition - single port serving both vendor and customer routes
resource "aws_ecs_task_definition" "main" {
  family                   = "installer-${var.nuon_id}"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task.arn

  container_definitions = jsonencode([
    {
      name      = "installer"
      image     = "${var.image_url}:${var.image_tag}"
      essential = true

      # Single port for all routes (vendor and customer use path-based routing)
      portMappings = [
        {
          containerPort = 8080
          hostPort      = 8080
          protocol      = "tcp"
        }
      ]

      environment = [
        {
          name  = "PORT"
          value = "8080"
        },
        {
          name  = "DATABASE_URL"
          value = var.database_url
        },
        {
          name  = "JWT_SECRET"
          value = var.jwt_secret
        },
        {
          name  = "CUSTOMER_BASE_URL"
          value = var.customer_base_url
        }
      ]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.service.name
          "awslogs-region"        = var.region
          "awslogs-stream-prefix" = "ecs"
        }
      }

      # Health check on single port
      healthCheck = {
        command     = ["CMD-SHELL", "curl -f http://localhost:8080/health || exit 1"]
        interval    = 30
        timeout     = 5
        retries     = 3
        startPeriod = 60
      }
    }
  ])

  tags = {
    Name   = "installer-${var.nuon_id}"
    NuonID = var.nuon_id
  }
}

# ECS Service - registers with BOTH target groups
resource "aws_ecs_service" "main" {
  name            = "installer-${var.nuon_id}"
  cluster         = var.cluster_arn
  task_definition = aws_ecs_task_definition.main.arn
  desired_count   = var.desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = data.aws_subnets.private.ids
    security_groups  = [aws_security_group.ecs.id]
    assign_public_ip = false
  }

  # Register with vendor ALB target group (same port, different domain)
  load_balancer {
    target_group_arn = var.vendor_target_group_arn
    container_name   = "installer"
    container_port   = 8080
  }

  # Register with customer ALB target group (same port, different domain)
  load_balancer {
    target_group_arn = var.customer_target_group_arn
    container_name   = "installer"
    container_port   = 8080
  }

  deployment_minimum_healthy_percent = 100
  deployment_maximum_percent         = 200

  tags = {
    Name   = "installer-${var.nuon_id}"
    NuonID = var.nuon_id
  }

  lifecycle {
    ignore_changes = [desired_count]
  }
}
