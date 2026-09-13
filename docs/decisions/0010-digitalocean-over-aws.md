# Deploy to DigitalOcean, not AWS

## Decision

Run Nightman on DigitalOcean (Droplet + Managed Load Balancer + Managed
Database), not AWS, for v1. The application architecture itself stays
cloud-agnostic (standard Postgres, stdlib HTTP, no DigitalOcean SDK calls
in application code) so a later port is a Terraform change, not an
application rewrite.

## Context

Nightman is a self-funded portfolio project. AWS experience is a more
common job-posting keyword than DigitalOcean, which was the initial pull
toward AWS — but the two have materially different cost shapes for a
deployment this small.

## Alternatives

- **AWS (EC2 + ALB + RDS).** Stronger resume signal for AWS-specific job
  requirements. Costed out at roughly $48–50/mo without a NAT Gateway, or
  $80–82/mo with one for a true private-subnet egress path — versus
  DigitalOcean's flat ~$33/mo for the equivalent Droplet + Load Balancer
  + Managed Database. AWS does have a 12-month free tier on EC2/RDS
  (t3/db.t3.micro) that would narrow this for a fresh account's first
  year, but the steady-state gap is real. Rejected for v1 on cost, given
  this is funded out of pocket for a project with no revenue.
- **AWS Lightsail.** Much closer to DigitalOcean's cost/complexity
  (~$20/mo), but a weaker signal for job postings that specifically want
  VPC/EC2/RDS/ALB-level experience — it trades away the thing AWS was
  being considered for in the first place.

## Rationale

For a project whose AWS value is mostly about being able to say "I've
used AWS" on a resume, the cost premium of doing that properly (real
VPC/EC2/RDS/ALB, not Lightsail) isn't justified when DigitalOcean
delivers the same architecture and security properties for less. Keeping
the application layer provider-neutral preserves the option to add an
AWS Terraform target later without revisiting the app.

## Consequences

- `deployments/terraform` targets DigitalOcean only for now; AWS
  portability is tracked as a future improvement (formalizing the
  provider-neutrality as an enforced rule, and actually standing up a
  second Terraform target) rather than built today — see
  [docs/plans/deployment.md](../plans/deployment.md).
- The "Design decisions" framing for this choice (cost comparison,
  trade-offs) lives here rather than being re-derived if the provider
  question comes up again later.
