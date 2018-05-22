package amicopy

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/hashicorp/packer/common"
)

// AmiCopy holds data and methods related to copying an image.
type AmiCopy struct {
	TargetAccountID string
	EC2             *ec2.EC2
	Input           *ec2.CopyImageInput
	Output          *ec2.CopyImageOutput
	SourceImage     *ec2.Image
}

// Copy will perform an EC2 copy based on the `Input` field.
// It will also call Tag to copy the source tags, if any.
func (ac *AmiCopy) Copy() (err error) {
	if err = ac.Input.Validate(); err != nil {
		return err
	}

	if ac.Output, err = ac.EC2.CopyImage(ac.Input); err != nil {
		return err
	}

	return ac.Tag()
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
