# NOTE: See https://www.packer.io/docs/builders/amazon/ebs for a full example.

packer {
  required_plugins {
    ami-copy = {
      version = ">=v1.7.0"
      source  = "github.com/martinbaillie/ami-copy"
    }
  }
}

source "amazon-ebs" "example" {
  ami_description = "Just a very basic example."
  ami_users       = "${var.aws_ami_users}"
  ami_name        = "${var.aws_ami_name}"

  snapshot_users = "${var.aws_ami_users}"
  ssh_username   = "root"
  instance_type  = "t3.micro"

  tags = {
    Name         = "${var.aws_ami_name}-${local.timestamp}"
    "ami:source" = "{{ .SourceAMI }}"
  }

  source_ami_filter {
    filters = {
      virtualization-type = "hvm"
      name                = "amzn2-ami-hvm-*-x86_64-ebs"
      root-device-type    = "ebs"
    }
    most_recent = true
    owners      = ["amazon"]
  }
}

build {
  sources = ["source.amazon-ebs.example"]

  post-processor "ami-copy" {
    ami_users    = "${var.aws_ami_users}"
    encrypt_boot = true
    role_name    = "AMICopyRole"
    // ... other settings.
  }
}
