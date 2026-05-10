package amicopy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"

	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/retry"
)

// AmiCopy defines the interface to copy images
type AmiCopy interface {
	Copy(ctx context.Context, ui *packer.Ui) error
	Input() *ec2.CopyImageInput
	Output() *ec2.CopyImageOutput
	Tag(ctx context.Context) error
	TargetAccountID() string
}

// AmiCopyImpl holds data and methods related to copying an image.
type AmiCopyImpl struct {
	targetAccountID string
	EC2             *ec2.Client
	input           *ec2.CopyImageInput
	output          *ec2.CopyImageOutput
	SourceImage     *ec2types.Image
	EnsureAvailable bool
	KeepArtifact    bool
	TagsOnly        bool
}

// AmiManifest holds the data about the resulting copied image
type AmiManifest struct {
	AccountID string `json:"account_id"`
	Region    string `json:"region"`
	ImageID   string `json:"image_id"`
}

// Copy will perform an EC2 copy based on the `Input` field.
// It will also call Tag to copy the source tags, if any.
func (ac *AmiCopyImpl) Copy(ctx context.Context, ui *packer.Ui) (err error) {
	if !ac.TagsOnly {
		if ac.output, err = ac.EC2.CopyImage(ctx, ac.input); err != nil {
			return err
		}
	} else {
		(*ui).Say(fmt.Sprintf("Only copying tags in %s as tags_only=true", ac.targetAccountID))
		ac.output = &ec2.CopyImageOutput{ImageId: ac.input.SourceImageId}
	}

	if err = ac.Tag(ctx); err != nil {
		return err
	}

	if ac.EnsureAvailable {
		(*ui).Say("Going to wait for image to be in available state")
		for i := 1; i <= 30; i++ {
			image, err := LocateSingleAMI(ctx, aws.ToString(ac.output.ImageId), ac.EC2)
			if err != nil && image == nil {
				return err
			}
			if image.State == ec2types.ImageStateAvailable {
				return nil
			}
			(*ui).Say(fmt.Sprintf("Waiting one minute (%d/30) for AMI to become available, current state: %s for image %s on account %s", i, image.State, *image.ImageId, ac.targetAccountID))
			time.Sleep(time.Duration(1) * time.Minute)
		}
		return fmt.Errorf("Timed out waiting for image %s to copy to account %s", *ac.output.ImageId, ac.targetAccountID)
	}

	return nil
}

func (ac *AmiCopyImpl) Input() *ec2.CopyImageInput {
	return ac.input
}

func (ac *AmiCopyImpl) SetInput(input *ec2.CopyImageInput) {
	ac.input = input
}

func (ac *AmiCopyImpl) Output() *ec2.CopyImageOutput {
	return ac.output
}

func (ac *AmiCopyImpl) TargetAccountID() string {
	return ac.targetAccountID
}

func (ac *AmiCopyImpl) SetTargetAccountID(id string) {
	ac.targetAccountID = id
}

// Tag will copy tags from the source image to the target (if any).
func (ac *AmiCopyImpl) Tag(ctx context.Context) (err error) {
	if len(ac.SourceImage.Tags) == 0 {
		return nil
	}

	// Retry creating tags for about 2.5 minutes
	return retry.Config{
		Tries: 11,
		ShouldRetry: func(err error) bool {
			var ae smithy.APIError
			if errors.As(err, &ae) {
				return ae.ErrorCode() == "UnauthorizedOperation"
			}
			return false
		},
		RetryDelay: (&retry.Backoff{InitialBackoff: 200 * time.Millisecond, MaxBackoff: 30 * time.Second, Multiplier: 2}).Linear,
	}.Run(ctx, func(ctx context.Context) error {
		_, err := ac.EC2.CreateTags(ctx, &ec2.CreateTagsInput{
			Resources: []string{aws.ToString(ac.output.ImageId)},
			Tags:      ac.SourceImage.Tags,
		})

		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "InvalidAMIID.NotFound" ||
				ae.ErrorCode() == "InvalidSnapshot.NotFound" {
				return nil
			}
		}

		return err
	})
}

// LocateSingleAMI tries to locate a single AMI for the given ID.
func LocateSingleAMI(ctx context.Context, id string, ec2Conn *ec2.Client) (*ec2types.Image, error) {
	if output, err := ec2Conn.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("image-id"),
				Values: []string{id},
			},
		},
	}); err != nil {
		return nil, err
	} else if len(output.Images) != 1 {
		return nil, fmt.Errorf("Single source image not located (found: %d images)",
			len(output.Images))
	} else {
		return &output.Images[0], nil
	}
}
