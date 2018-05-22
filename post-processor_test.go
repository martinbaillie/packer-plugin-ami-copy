package main

import (
	"bytes"
	"testing"

	awscommon "github.com/hashicorp/packer/builder/amazon/common"
	"github.com/hashicorp/packer/builder/amazon/ebs"
	"github.com/hashicorp/packer/packer"
)

func TestPostProcessor_ImplementsPostProcessor(t *testing.T) {
	var _ packer.PostProcessor = new(PostProcessor)
}

// TODO: AWS API mocking

// Stubs
func stubConfig() map[string]interface{} {
	return map[string]interface{}{
		"ami_users": []string{
			"123456789104",
			"123456789103",
			"123456789102",
			"123456789101",
		},
		"role_name":    "AMICopyRole",
		"encrypt_boot": true,
	}
}

func stubPP(t *testing.T) *PostProcessor {
	var p PostProcessor
	if err := p.Configure(stubConfig()); err != nil {
		t.Fatalf("err: %s", err)
	}
	return &p
}

func stubUI() *packer.BasicUi {
	return &packer.BasicUi{
		Reader: new(bytes.Buffer),
		Writer: new(bytes.Buffer),
	}
}

func stubArtifact() packer.Artifact {
	artifact := &awscommon.Artifact{
		BuilderIdValue: ebs.BuilderId,
		Amis: map[string]string{
			"ap-southeast-2": "ami-d2a666b0",
			"us-west-1":      "ami-d23104f1",
		},
	}

	return artifact
}
