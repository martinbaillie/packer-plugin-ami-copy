//go:generate mapstructure-to-hcl2 -type Config

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/hashicorp/hcl/v2/hcldec"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/hashicorp/packer/builder/amazon/chroot"
	"github.com/hashicorp/packer/builder/amazon/ebs"
	"github.com/hashicorp/packer/builder/amazon/ebssurrogate"
	"github.com/hashicorp/packer/builder/amazon/ebsvolume"
	"github.com/hashicorp/packer/builder/amazon/instance"

	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"

	"github.com/martinbaillie/packer-plugin-ami-copy/amicopy"

	awscommon "github.com/hashicorp/packer/builder/amazon/common"
)

// BuilderId is the ID of this post processor.
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
	EnsureAvailable bool   `mapstructure:"ensure_available"`
	KeepArtifact    string `mapstructure:"keep_artifact"`
	ManifestOutput  string `mapstructure:"manifest_output"`

	ctx interpolate.Context
}

// PostProcessor implements Packer's PostProcessor interface.
type PostProcessor struct {
	config Config
}

func (p *PostProcessor) ConfigSpec() hcldec.ObjectSpec {
	return p.config.FlatMapstructure().HCL2Spec()
}

// Configure interpolates and validates requisite vars for the PostProcessor.
func (p *PostProcessor) Configure(raws ...interface{}) error {
	p.config.ctx.Funcs = awscommon.TemplateFuncs

	if err := config.Decode(&p.config, &config.DecodeOpts{
		PluginType:         BuilderId,
		Interpolate:        true,
		InterpolateContext: &p.config.ctx,
		InterpolateFilter: &interpolate.RenderFilter{
			Exclude: []string{},
		},
	}, raws...); err != nil {
		return err
	}

	if len(p.config.AMIUsers) == 0 {
		return errors.New("ami_users must be set")
	}

	if len(p.config.KeepArtifact) == 0 {
		p.config.KeepArtifact = "true"
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
	ctx context.Context, ui packer.Ui, artifact packer.Artifact) (packer.Artifact, bool, bool, error) {

	keepArtifactBool, err := strconv.ParseBool(p.config.KeepArtifact)
	if err != nil {
		return artifact, keepArtifactBool, false, err
	}

	// Ensure we're being called from a supported builder
	switch artifact.BuilderId() {
	case ebs.BuilderId,
		ebssurrogate.BuilderId,
		ebsvolume.BuilderId,
		chroot.BuilderId,
		instance.BuilderId:
		break
	default:
		return artifact, keepArtifactBool, false,
			fmt.Errorf("Unexpected artifact type: %s\nCan only export from Amazon builders",
				artifact.BuilderId())
	}

	// Current AWS session
	currSession, err := p.config.AccessConfig.Session()
	if err != nil {
		return artifact, keepArtifactBool, false, err
	}

	// Copy futures
	var (
		amis   = amisFromArtifactID(artifact.Id())
		users  = p.config.AMIUsers
		copies []amicopy.AmiCopy
	)
	for _, ami := range amis {
		var source *ec2.Image
		if source, err = amicopy.LocateSingleAMI(
			ami.id,
			ec2.New(currSession, aws.NewConfig().WithRegion(ami.region)),
		); err != nil || source == nil {
			return artifact, keepArtifactBool, false, err
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
				if source.Name != nil {
					name = *source.Name
				}
				if source.Description != nil {
					description = *source.Description
				}
			}

			amiCopy := &amicopy.AmiCopyImpl{
				EC2:             conn,
				SourceImage:     source,
				EnsureAvailable: p.config.EnsureAvailable,
			}
			amiCopy.SetTargetAccountID(user)
			amiCopy.SetInput(&ec2.CopyImageInput{
				Name:          aws.String(name),
				Description:   aws.String(description),
				SourceImageId: aws.String(ami.id),
				SourceRegion:  aws.String(ami.region),
				KmsKeyId:      aws.String(p.config.AMIKmsKeyId),
				Encrypted:     aws.Bool(*p.config.AMIEncryptBootVolume.ToBoolPointer()),
			})

			copies = append(copies, amiCopy)
		}
	}

	copyErrs := copyAMIs(copies, ui, p.config.ManifestOutput, p.config.CopyConcurrency)
	if copyErrs > 0 {
		return artifact, true, false, fmt.Errorf(
			"%d/%d AMI copies failed, manual reconciliation may be required", copyErrs, len(copies))
	}

	return artifact, keepArtifactBool, false, nil
}

func copyAMIs(copies []amicopy.AmiCopy, ui packer.Ui, manifestOutput string, concurrencyCount int) int32 {
	// Copy execution loop
	var (
		copyCount    = len(copies)
		copyTasks    = make(chan amicopy.AmiCopy, copyCount)
		amiManifests = make(chan *amicopy.AmiManifest, copyCount)
		copyErrs     int32
		wg           sync.WaitGroup
	)
	var workers int
	{
		if workers = concurrencyCount; workers == 0 {
			workers = copyCount
		}
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range copyTasks {
				input := c.Input()
				ui.Say(
					fmt.Sprintf(
						"[%s] Copying %s to account %s (encrypted: %t)",
						*input.SourceRegion,
						*input.SourceImageId,
						c.TargetAccountID(),
						*input.Encrypted,
					),
				)
				if err := c.Copy(&ui); err != nil {
					ui.Say(err.Error())
					atomic.AddInt32(&copyErrs, 1)
					continue
				}
				output := c.Output()
				manifest := &amicopy.AmiManifest{
					AccountID: c.TargetAccountID(),
					Region:    *input.SourceRegion,
					ImageID:   *output.ImageId,
				}
				amiManifests <- manifest

				ui.Say(
					fmt.Sprintf(
						"[%s] Finished copying %s to %s (copied id: %s)",
						*input.SourceRegion,
						*input.SourceImageId,
						c.TargetAccountID(),
						*output.ImageId,
					),
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

	if manifestOutput != "" {
		manifests := []*amicopy.AmiManifest{}
	LOOP:
		for {
			select {
			case m := <-amiManifests:
				manifests = append(manifests, m)
			default:
				break LOOP
			}
		}
		err := writeManifests(manifestOutput, manifests)
		if err != nil {
			ui.Say(fmt.Sprintf("Unable to write out manifest to %s: %s", manifestOutput, err))
		}
	}
	close(amiManifests)

	return copyErrs
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

func writeManifests(output string, manifests []*amicopy.AmiManifest) error {
	rawManifest, err := json.Marshal(manifests)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(output, rawManifest, 0644)
}
