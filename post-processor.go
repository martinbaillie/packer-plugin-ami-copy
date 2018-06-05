package main

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/martinbaillie/packer-post-processor-ami-copy/amicopy"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/hashicorp/packer/builder/amazon/chroot"
	"github.com/hashicorp/packer/builder/amazon/ebs"
	"github.com/hashicorp/packer/builder/amazon/ebssurrogate"
	"github.com/hashicorp/packer/builder/amazon/ebsvolume"
	"github.com/hashicorp/packer/builder/amazon/instance"
	"github.com/hashicorp/packer/common"
	"github.com/hashicorp/packer/helper/config"
	"github.com/hashicorp/packer/packer"
	"github.com/hashicorp/packer/template/interpolate"

	awscommon "github.com/hashicorp/packer/builder/amazon/common"
)

// BuilderId is the ID of this post processer.
// nolint: golint
const BuilderId = "packer.post-processor.ami-copy"

// Config is the post-processor configuration with interpolation supported.
// See https://www.packer.io/docs/builders/amazon.html for details.
type Config struct {
	common.PackerConfig    `mapstructure:",squash"`
	awscommon.AccessConfig `mapstructure:",squash"`
	awscommon.AMIConfig    `mapstructure:",squash"`

	// Variables specific to this post-processor
	RoleName        string `mapstructure:"role_name"`
	CopyConcurrency int    `mapstructure:"copy_concurrency"`

	ctx interpolate.Context
}

// PostProcessor implements Packer's PostProcessor interface.
type PostProcessor struct {
	config Config
}

// Configure interpolates and validates requisite vars for the PostProcessor.
func (p *PostProcessor) Configure(raws ...interface{}) error {
	p.config.ctx.Funcs = awscommon.TemplateFuncs
	err := config.Decode(&p.config, &config.DecodeOpts{
		Interpolate:        true,
		InterpolateContext: &p.config.ctx,
		InterpolateFilter: &interpolate.RenderFilter{
			Exclude: []string{},
		},
	}, raws...)
	if err != nil {
		return err
	}

	if len(p.config.AMIUsers) == 0 {
		return errors.New("ami_users must be set")
	}

	return nil
}

// PostProcess will copy the source AMI to each of the target accounts as
// designated by the mandatory `ami_users` variable. It will optionally
// encrypt the copied AMIs (`encrypt_boot`) with `kms_key_id` if set, or the
// default EBS KMS key if unset. Tags will be copied with the image.
//
// Copies are executed concurrently. This concurrency is unlimited unless
// controller by `copy_concurrency`.
func (p *PostProcessor) PostProcess(
	ui packer.Ui, artifact packer.Artifact) (packer.Artifact, bool, error) {
	// Ensure we're being called from a supported builder
	switch artifact.BuilderId() {
	case ebs.BuilderId,
		ebssurrogate.BuilderId,
		ebsvolume.BuilderId,
		chroot.BuilderId,
		instance.BuilderId:
		break
	default:
		return artifact, true,
			fmt.Errorf("Unexpected artifact type: %s\nCan only export from Amazon builders",
				artifact.BuilderId())
	}

	// Current AWS session
	var currSession, err = p.config.AccessConfig.Session()
	if err != nil {
		return artifact, true, err
	}

	// Copy futures
	var (
		amis   = amisFromArtifactID(artifact.Id())
		users  = p.config.AMIUsers
		copies []*amicopy.AmiCopy
	)
	for _, ami := range amis {
		var source *ec2.Image
		if source, err = amicopy.LocateSingleAMI(ami.id, ec2.New(currSession)); err != nil ||
			source == nil {
			return artifact, true, err
		}

		for _, user := range users {
			var conn *ec2.EC2
			{
				if p.config.RoleName != "" {
					var (
						role = fmt.Sprintf("arn:aws:iam::%s:role/%s", user, p.config.RoleName)
						sess = currSession.Copy(&aws.Config{Region: aws.String(ami.region)})
					)
					conn = ec2.New(sess, &aws.Config{
						Credentials: stscreds.NewCredentials(sess, role),
					})
				} else {
					conn = ec2.New(currSession.Copy(&aws.Config{Region: aws.String(ami.region)}))
				}
			}

			var name, description string
			{
				if *source.Name != "" {
					name = *source.Name
				}
				if *source.Description != "" {
					description = *source.Description
				}
			}

			copies = append(copies, &amicopy.AmiCopy{
				EC2:             conn,
				SourceImage:     source,
				TargetAccountID: user,
				Input: &ec2.CopyImageInput{
					Name:          aws.String(name),
					Description:   aws.String(description),
					SourceImageId: aws.String(ami.id),
					SourceRegion:  aws.String(ami.region),
					KmsKeyId:      aws.String(p.config.AMIKmsKeyId),
					Encrypted:     aws.Bool(p.config.AMIEncryptBootVolume),
				},
			})
		}
	}

	// Copy execution loop
	var (
		copyCount = len(copies)
		copyTasks = make(chan *amicopy.AmiCopy, copyCount)
		copyErrs  int32
		wg        sync.WaitGroup
	)
	var workers int
	{
		if workers = p.config.CopyConcurrency; workers == 0 {
			workers = copyCount
		}
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range copyTasks {
				ui.Say(fmt.Sprintf("[%s] Copying %s to account %s (encrypted: %t)",
					*c.Input.SourceRegion,
					*c.Input.SourceImageId,
					c.TargetAccountID,
					*c.Input.Encrypted),
				)
				if err = c.Copy(); err != nil {
					ui.Say(err.Error())
					atomic.AddInt32(&copyErrs, 1)
					continue
				}
				ui.Say(fmt.Sprintf("[%s] Finished copying %s to %s (copied id: %s)",
					*c.Input.SourceRegion,
					*c.Input.SourceImageId,
					c.TargetAccountID,
					*c.Output.ImageId),
				)
			}
		}()
	}

	// Copy task submission
	for _, copy := range copies {
		copyTasks <- copy
	}
	close(copyTasks)
	wg.Wait()

	if copyErrs > 0 {
		return artifact, true, fmt.Errorf(
			"%d/%d AMI copies failed, manual reconciliation may be required", copyErrs, copyCount)
	}
	return artifact, true, nil
}

// ami encapsulates simplistic details about an AMI.
type ami struct {
	id     string
	region string
}

// amisFromArtifactID returns an AMI slice from a Packer artifact id.
func amisFromArtifactID(artifactID string) (amis []*ami) {
	for _, amiStr := range strings.Split(artifactID, ",") {
		pair := strings.SplitN(amiStr, ":", 2)
		amis = append(amis, &ami{region: pair[0], id: pair[1]})
	}
	return amis
}
