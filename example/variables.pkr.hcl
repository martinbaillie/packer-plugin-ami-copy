locals {
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
}

variable "aws_ami_users" {
  default = ["123456789012", "456789012345"]
}

variable "aws_ami_name" {
  default = "example"
}

# - `ami_users` (array of strings) - A list of account IDs to copy the images to. NOTE: you must share AMI and snapshot access in the builder through `ami_users` and `snapshot_users` respectively.

# Optional:

# - `copy_concurrency` (integer) - Limit the number of copies executed in parallel (default: unlimited).
# - `encrypt_boot` (boolean) - create the copy with an encrypted EBS volume in the target accounts
# - `kms_key_id` (string) - the ID of the KMS key to use for boot volume encryption. (default EBS KMS key used otherwise).
# - `ensure_available` (boolean) - wait until the AMI becomes available in the copy target account(s)
# - `keep_artifact` (boolean) - remove the original generated AMI after copy (default: true)
# - `manifest_output` (string) - the name of the file we output AMI IDs to, in JSON format (default: no manifest file is written)
