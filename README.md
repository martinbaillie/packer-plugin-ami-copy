[![License](https://img.shields.io/badge/license-BSD-brightgreen.svg?style=flat-square)](/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/martinbaillie/packer-post-processor-ami-copy?style=flat-square)](https://goreportcard.com/report/github.com/martinbaillie/packer-post-processor-ami-copy)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](http://godoc.org/github.com/martinbaillie/packer-post-processor-ami-copy)
[![Build](https://img.shields.io/travis/martinbaillie/packer-post-processor-ami-copy/master.svg?style=flat-square)](https://travis-ci.org/martinbaillie/packer-post-processor-ami-copy)
[![Release](https://img.shields.io/github/release/martinbaillie/packer-post-processor-ami-copy.svg?style=flat-square)](https://github.com/martinbaillie/packer-post-processor-ami-copy/releases/latest)

# packer-post-processor-ami-copy

### Description

This plugin fills a gap in a lot of AWS image bakery workflows where the source image built by any of Packer's Amazon builders (EBS, Chroot, Instance etc.) needs to be copied to a number of target accounts.

For each `region:ami-id` built, the plugin will copy the image and tags, and optionally encrypt the target AMI and wait for it to become active.

### Installation

This is a packer _plugin_. Please read the plugin [documentation](https://www.packer.io/docs/extend/plugins.html).

You can download the latest binary for your architecture from the [releases page](https://github.com/martinbaillie/packer-post-processor-ami-copy/releases/latest).

### Usage

```json
"builders": [
  {
    "type": "amazon-ebs",
    "ami_users": "{{user `aws_ami_users`}}",
    "snapshot_users": "{{user `aws_ami_users`}}",
    "tags": {
      "Name": "{{user `aws_ami_name`}}-{{timestamp}}",
      "ami:source": "{{.SourceAMI}}",
    }
  }
],
"provisioners": [],
"post-processors": [
  {
    "type": "ami-copy",
    "ami_users":"{{user `aws_ami_users`}}",
    "role_name":    "AMICopyRole",
    "encrypt_boot": true
  }
]
```

### Configuration

Type: `ami-copy`

Required:

- `ami_users` (array of strings) - A list of account IDs to copy the images to. NOTE: you must share AMI and snapshot access in the builder through `ami_users` and `snapshot_users` respectively.

Optional:

- `copy_concurrency` (integer) - Limit the number of copies executed in parallel (default: unlimited).
- `encrypt_boot` (boolean) - create the copy with an encrypted EBS volume in the target accounts
- `kms_key_id` (string) - the ID of the KMS key to use for boot volume encryption. (default EBS KMS key used otherwise).
- `ensure_available` (boolean) - wait until the AMI becomes available in the copy target account(s)
- `keep_artifact` (boolean) - remove the original generated AMI after copy (default: true)
- `manifest_output` (string) - the name of the file we output AMI IDs to, in JSON format (default: no manifest file is written)
