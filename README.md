[![License](https://img.shields.io/badge/license-BSD-brightgreen.svg?style=flat-square)](/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/martinbaillie/packer-plugin-ami-copy?style=flat-square)](https://goreportcard.com/report/github.com/martinbaillie/packer-plugin-ami-copy)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](http://godoc.org/github.com/martinbaillie/packer-plugin-ami-copy)
[![Build](https://github.com/martinbaillie/packer-plugin-ami-copy/actions/workflows/build.yml/badge.svg)](https://github.com/martinbaillie/packer-plugin-ami-copy/actions/workflows/build.yml)
[![Release](https://github.com/martinbaillie/packer-plugin-ami-copy/actions/workflows/release.yml/badge.svg)](https://github.com/martinbaillie/packer-plugin-ami-copy/actions/workflows/release.yml)

# packer-plugin-ami-copy

## Description

This plugin fills a gap in a lot of AWS image bakery workflows where the source
image built by any of Packer's Amazon builders (EBS, Chroot, Instance etc.)
needs to be copied to a number of target accounts.

For each `region:ami-id` built, the plugin will copy the image and tags, and
optionally encrypt the target AMI and wait for it to become active.

## Installation

### Using pre-built releases

#### Using the `packer init` command

Starting from version 1.7, Packer supports a new `packer init` command allowing
automatic installation of Packer plugins. Read the [Packer
documentation][packer-doc-init] for more information.

#### Manual installation

You can find pre-built binary releases of the plugin [here][releases].
Once you have downloaded the latest archive corresponding to your target OS,
uncompress it to retrieve the plugin binary file corresponding to your platform.
To install the plugin, please follow the Packer documentation on
[installing a plugin][packer-doc-plugins].

You can and should verify the authenticity and integrity of the plugin you
downloaded. All released binaries are hashed and the resulting sums are signed
by my GPG key.

```sh
# Import my key.
curl -sS https://github.com/martinbaillie.gpg | gpg --import -

# Verify the authenticity.
gpg --verify SHA256SUMS.sig SHA256SUMS

# Verify the integrity.
shasum -a 256 -c SHA256SUMS
```

### From Sources

If you prefer to build the plugin from sources you will need a modern Go
compiler toolchain (Go 1.16+ ideally). If you are a Nix user,
[`shell.nix`](shell.nix) can be of use here.

Clone the GitHub repository locally then run a `go build` from the root. Upon
successful compilation, a `packer-plugin-ami-copy` plugin binary will be
produced. To install it, follow the official Packer documentation on
[installing a plugin][packer-doc-plugins].

## Usage

For more information on how to use the plugin, see the [`docs/`](docs) and
[`examples/`](examples).

## Configuration

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
- `tags_only` (boolean) - if set to `true`, then the AMI won't be copied, but the tags will be duplicated on the shared AMI in the destination account.

[packer-doc-plugins]: https://www.packer.io/docs/extending/plugins/#installing-plugins
[packer-doc-init]: https://www.packer.io/docs/commands/init
[packer-doc-plugins]: https://www.packer.io/docs/extending/plugins/#installing-plugins
[packer]: https://www.packer.io/
[releases]: https://github.com/martinbaillie/packer-plugin-ami-copy/releases
