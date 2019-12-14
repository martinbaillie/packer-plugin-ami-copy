package main

import (
	"bytes"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	awscommon "github.com/hashicorp/packer/builder/amazon/common"
	"github.com/hashicorp/packer/builder/amazon/ebs"
	"github.com/hashicorp/packer/packer"
	"github.com/martinbaillie/packer-post-processor-ami-copy/amicopy"
)

func TestPostProcessor_ImplementsPostProcessor(t *testing.T) {
	var _ packer.PostProcessor = new(PostProcessor)
}

func TestConcurrentCopy(t *testing.T) {
	ui := &packer.BasicUi{
		Reader: new(bytes.Buffer),
		Writer: new(bytes.Buffer),
	}
	copies := []amicopy.AmiCopy{
		&MockAmiCopy{},
		&MockAmiCopy{},
	}
	errs := copyAMIs(copies, ui, "", 2)
	if errs > 0 {
		t.Fatalf("Too many errors %d", errs)
	}
}

func TestConcurrentCopyWithManifest(t *testing.T) {
	ui := &packer.BasicUi{
		Reader: new(bytes.Buffer),
		Writer: new(bytes.Buffer),
	}
	copies := []amicopy.AmiCopy{
		&MockAmiCopy{},
		&MockAmiCopy{},
	}
	errs := copyAMIs(copies, ui, "testdata/manifest.json", 2)
	if errs > 0 {
		t.Fatalf("Too many errors %d", errs)
	}
}

type MockAmiCopy struct{}

func (m *MockAmiCopy) Copy(ui *packer.Ui) error {
	return nil
}

func (m *MockAmiCopy) Input() *ec2.CopyImageInput {
	return &ec2.CopyImageInput{
		SourceRegion:  aws.String("ap-southeast-2"),
		SourceImageId: aws.String("ami-abcd1234"),
		Encrypted:     aws.Bool(true),
	}
}

func (m *MockAmiCopy) Output() *ec2.CopyImageOutput {
	return &ec2.CopyImageOutput{
		ImageId: aws.String("ami-1234abcd"),
	}
}

func (m *MockAmiCopy) Tag() error {
	return nil
}

func (m *MockAmiCopy) TargetAccountID() string {
	return "012345678910"
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
