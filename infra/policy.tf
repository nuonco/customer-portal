data "aws_iam_policy_document" "rds_iam_auth" {
  statement {
    effect = "Allow"
    actions = [
      "rds-db:connect",
    ]
    resources = [
      format("arn:aws:rds-db:%s:%s:dbuser:%s/%s",
        local.vars.region,
        local.accounts[var.env].id,
        module.primary.db_instance_resource_id,
        "customer_dashboard",
      ),
    ]
  }
}

resource "aws_iam_policy" "rds_iam_auth" {
  name   = "eks-policy-${local.name}-rds-iam-auth"
  policy = data.aws_iam_policy_document.rds_iam_auth.json
}
