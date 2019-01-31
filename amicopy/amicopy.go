package amicopy

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"

	"errors"

	"github.com/hashicorp/packer/common"
	"github.com/hashicorp/packer/packer"
)

// AmiCopy holds data and methods related to copying an image.
type AmiCopy struct {
	TargetAccountID string
	EC2             *ec2.EC2
	Input           *ec2.CopyImageInput
	Output          *ec2.CopyImageOutput
	SourceImage     *ec2.Image
	EnsureAvailable bool
	KeepArtifact    bool
}

// Copy will perform an EC2 copy based on the `Input` field.
// It will also call Tag to copy the source tags, if any.
func (ac *AmiCopy) Copy(ui *packer.Ui) (err error) {
	if err = ac.Input.Validate(); err != nil {
		return err
	}

	if ac.Output, err = ac.EC2.CopyImage(ac.Input); err != nil {
		return err
	}

	if err = ac.Tag(); err != nil {
		return err
	}

	if ac.EnsureAvailable {
		(*ui).Say("Going to wait for image to be in available state")
		for i := 1; i <= 30; i++ {
			image, err := LocateSingleAMI(*ac.Output.ImageId, ac.EC2)
			if err != nil && image == nil {
				return err
			}
			if *image.State == ec2.ImageStateAvailable {
				return nil
			}
			(*ui).Say(fmt.Sprintf("Waiting one minute (%d/30) for AMI to become available, current state: %s for image %s on account %s", i, *image.State, *image.ImageId, ac.TargetAccountID))
			time.Sleep(time.Duration(1) * time.Minute)
		}
		return errors.New(fmt.Sprintf("Timed out waiting for image %s to copy to account %s", *ac.Output.ImageId, ac.TargetAccountID))
	}

	return nil
}

// Tag will copy tags from the source image to the target (if any).
func (ac *AmiCopy) Tag() (err error) {
	if len(ac.SourceImage.Tags) == 0 {
		return nil
	}

	// Retry creating tags for about 2.5 minutes
	return common.Retry(0.2, 30, 11, func(i uint) (bool, error) {
		_, err := ac.EC2.CreateTags(&ec2.CreateTagsInput{
			Resources: []*string{ac.Output.ImageId},
			Tags:      ac.SourceImage.Tags,
		})

		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == "InvalidAMIID.NotFound" ||
				awsErr.Code() == "InvalidSnapshot.NotFound" {
				return false, nil
			}
		}

		return true, err
	})
}

// LocateSingleAMI tries to locate a single AMI for the given ID.
func LocateSingleAMI(id string, ec2Conn *ec2.EC2) (*ec2.Image, error) {
	if output, err := ec2Conn.DescribeImages(&ec2.DescribeImagesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("image-id"),
				Values: aws.StringSlice([]string{id}),
			},
		},
	}); err != nil {
		return nil, err
	} else if len(output.Images) != 1 {
		return nil, fmt.Errorf("Single source image not located (found: %d images)",
			len(output.Images))
	} else {
		return output.Images[0], nil
	}
}
