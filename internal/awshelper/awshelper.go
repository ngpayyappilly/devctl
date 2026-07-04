package awshelper

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spf13/cobra"

	"devctl/pkg/config"
	deverrors "devctl/pkg/errors"
)

func NewAwsHelperCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aws",
		Short: "Perform quick actions with AWS",
	}

	// S3 commands
	cmd.AddCommand(listS3Cmd())
	cmd.AddCommand(listBucketObjectsCmd())
	cmd.AddCommand(displayBucketPolicyCmd())
	// EC2 commands
	cmd.AddCommand(listEC2Cmd())
	cmd.AddCommand(displayEC2DetailsCmd())
	cmd.AddCommand(sshEC2Cmd())
	// CloudFormation commands
	cmd.AddCommand(listStacksCmd())
	cmd.AddCommand(deleteStackCmd())
	cmd.AddCommand(checkStackDriftCmd())
	// IAM commands
	cmd.AddCommand(listIAMUsersCmd())
	cmd.AddCommand(listIAMRolesCmd())
	cmd.AddCommand(listIAMPoliciesCmd())
	cmd.AddCommand(displayIAMRolePoliciesCmd())

	return cmd
}

func loadAWSConfig() (aws.Config, error) {
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return aws.Config{}, deverrors.NewConfigError("load AWS config: %v", err)
	}
	return cfg, nil
}

func listS3Cmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-s3",
		Short: "List all S3 buckets",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadAWSConfig()
			if err != nil {
				return err
			}
			result, err := s3.NewFromConfig(cfg).ListBuckets(context.TODO(), &s3.ListBucketsInput{})
			if err != nil {
				return deverrors.NewAPIError("list buckets: %v", err)
			}
			for _, bucket := range result.Buckets {
				fmt.Printf("🪣 %s\n", aws.ToString(bucket.Name))
			}
			return nil
		},
	}
}

func listBucketObjectsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-bucket-objects",
		Short: "List objects in an S3 bucket",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return deverrors.NewUsageError("bucket name is required")
			}
			cfg, err := loadAWSConfig()
			if err != nil {
				return err
			}
			result, err := s3.NewFromConfig(cfg).ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
				Bucket: aws.String(args[0]),
			})
			if err != nil {
				return deverrors.NewAPIError("list objects in %s: %v", args[0], err)
			}
			for _, object := range result.Contents {
				fmt.Printf("📦 %s\n", aws.ToString(object.Key))
			}
			return nil
		},
	}
}

func displayBucketPolicyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "display-bucket-policy",
		Short: "Display S3 bucket policy",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return deverrors.NewUsageError("bucket name is required")
			}
			cfg, err := loadAWSConfig()
			if err != nil {
				return err
			}
			result, err := s3.NewFromConfig(cfg).GetBucketPolicy(context.TODO(), &s3.GetBucketPolicyInput{
				Bucket: aws.String(args[0]),
			})
			if err != nil {
				return deverrors.NewAPIError("get bucket policy for %s: %v", args[0], err)
			}
			fmt.Printf("🪣 Bucket Policy for %s:\n%s\n", args[0], aws.ToString(result.Policy))
			return nil
		},
	}
}

func listEC2Cmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-ec2",
		Short: "List EC2 instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadAWSConfig()
			if err != nil {
				return err
			}
			output, err := ec2.NewFromConfig(cfg).DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{})
			if err != nil {
				return deverrors.NewAPIError("describe instances: %v", err)
			}
			for _, res := range output.Reservations {
				for _, inst := range res.Instances {
					fmt.Printf("🖥️ Instance ID: %s | State: %s | Type: %s\n",
						aws.ToString(inst.InstanceId),
						string(inst.State.Name),
						string(inst.InstanceType))
				}
			}
			return nil
		},
	}
}

func displayEC2DetailsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "display-ec2",
		Short: "Display EC2 instance details",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return deverrors.NewUsageError("instance ID is required")
			}
			cfg, err := loadAWSConfig()
			if err != nil {
				return err
			}
			output, err := ec2.NewFromConfig(cfg).DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{
				InstanceIds: []string{args[0]},
			})
			if err != nil {
				return deverrors.NewAPIError("describe instance %s: %v", args[0], err)
			}
			for _, res := range output.Reservations {
				for _, inst := range res.Instances {
					fmt.Printf("🖥️ Instance ID: %s | State: %s | Type: %s\n",
						aws.ToString(inst.InstanceId),
						string(inst.State.Name),
						string(inst.InstanceType))
				}
			}
			return nil
		},
	}
}

func listStacksCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-cf-stacks",
		Short: "List CloudFormation stacks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadAWSConfig()
			if err != nil {
				return err
			}
			resp, err := cloudformation.NewFromConfig(cfg).ListStacks(context.TODO(), &cloudformation.ListStacksInput{})
			if err != nil {
				return deverrors.NewAPIError("list stacks: %v", err)
			}
			for _, summary := range resp.StackSummaries {
				fmt.Printf("🧱 Stack: %s | Status: %s\n",
					aws.ToString(summary.StackName),
					string(summary.StackStatus))
			}
			return nil
		},
	}
}

func sshEC2Cmd() *cobra.Command {
	var instanceID string
	var keyPath string
	var username string
	var region string

	cmd := &cobra.Command{
		Use:   "ssh-ec2",
		Short: "SSH into an EC2 instance using Instance ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("region") {
				region = config.GetString(config.KeyAWSRegion, "")
				if region == "" {
					return deverrors.NewConfigError(
						"no AWS region configured: pass --region, set AWS_REGION, or add defaults.aws_region to ~/.devctl/config.yaml")
				}
			}
			if !cmd.Flags().Changed("user") {
				username = config.GetString(config.KeySSHUsername, "")
			}

			if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
				fmt.Fprintf(cmd.OutOrStdout(),
					"[dry-run] Would SSH into instance %s (key: %s, user: %s, region: %s)\n",
					instanceID, keyPath, username, region)
				return nil
			}

			cfg, err := awsconfig.LoadDefaultConfig(context.TODO(), awsconfig.WithRegion(region))
			if err != nil {
				return deverrors.NewConfigError("load AWS config: %v", err)
			}

			out, err := ec2.NewFromConfig(cfg).DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{
				InstanceIds: []string{instanceID},
			})
			if err != nil || len(out.Reservations) == 0 || len(out.Reservations[0].Instances) == 0 {
				return deverrors.NewAPIError("describe instance %s: %v", instanceID, err)
			}

			inst := out.Reservations[0].Instances[0]
			if inst.PublicIpAddress == nil {
				return deverrors.NewAPIError("instance %s does not have a public IP", instanceID)
			}

			ip := aws.ToString(inst.PublicIpAddress)
			sshCmd := fmt.Sprintf("ssh -i %s %s@%s", keyPath, username, ip)
			fmt.Printf("👉 Executing: %s\n", sshCmd)
			if err := executeShell(sshCmd); err != nil {
				return fmt.Errorf("SSH command failed: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&instanceID, "instance-id", "i", "", "EC2 instance ID (required)")
	cmd.Flags().StringVarP(&keyPath, "key", "k", "", "Path to private key file (required)")
	cmd.Flags().StringVarP(&username, "user", "u", "", "SSH username (falls back to config, then ec2-user)")
	cmd.Flags().StringVar(&region, "region", "", "AWS region (falls back to config, then us-east-1)")
	cmd.MarkFlagRequired("instance-id")
	cmd.MarkFlagRequired("key")

	return cmd
}

func executeShell(command string) error {
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func deleteStackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete-cf-stack",
		Short: "Delete a CloudFormation stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return deverrors.NewUsageError("stack name is required")
			}
			if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] Would execute: DeleteStack { StackName: %q }\n", args[0])
				return nil
			}
			cfg, err := loadAWSConfig()
			if err != nil {
				return err
			}
			_, err = cloudformation.NewFromConfig(cfg).DeleteStack(context.TODO(), &cloudformation.DeleteStackInput{
				StackName: aws.String(args[0]),
			})
			if err != nil {
				return deverrors.NewAPIError("delete stack %s: %v", args[0], err)
			}
			fmt.Printf("✅ Stack %s deletion initiated.\n", args[0])
			return nil
		},
	}
}

func checkStackDriftCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check-stack-drift",
		Short: "Check if a CloudFormation stack is drifted",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return deverrors.NewUsageError("stack name is required")
			}
			cfg, err := loadAWSConfig()
			if err != nil {
				return err
			}
			_, err = cloudformation.NewFromConfig(cfg).DescribeStackResourceDrifts(context.TODO(), &cloudformation.DescribeStackResourceDriftsInput{
				StackName: aws.String(args[0]),
			})
			if err != nil {
				return deverrors.NewAPIError("check stack drift for %s: %v", args[0], err)
			}
			fmt.Printf("✅ Stack %s drift check initiated.\n", args[0])
			return nil
		},
	}
}

func listIAMUsersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-iam-users",
		Short: "List IAM users",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadAWSConfig()
			if err != nil {
				return err
			}
			result, err := iam.NewFromConfig(cfg).ListUsers(context.TODO(), &iam.ListUsersInput{})
			if err != nil {
				return deverrors.NewAPIError("list IAM users: %v", err)
			}
			for _, user := range result.Users {
				fmt.Printf("👤 %s\n", aws.ToString(user.UserName))
			}
			return nil
		},
	}
}

func listIAMRolesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-iam-roles",
		Short: "List IAM roles",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadAWSConfig()
			if err != nil {
				return err
			}
			result, err := iam.NewFromConfig(cfg).ListRoles(context.TODO(), &iam.ListRolesInput{})
			if err != nil {
				return deverrors.NewAPIError("list IAM roles: %v", err)
			}
			for _, role := range result.Roles {
				fmt.Printf("👤 %s\n", aws.ToString(role.RoleName))
			}
			return nil
		},
	}
}

func listIAMPoliciesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-iam-policies",
		Short: "List IAM policies",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadAWSConfig()
			if err != nil {
				return err
			}
			result, err := iam.NewFromConfig(cfg).ListPolicies(context.TODO(), &iam.ListPoliciesInput{})
			if err != nil {
				return deverrors.NewAPIError("list IAM policies: %v", err)
			}
			for _, policy := range result.Policies {
				fmt.Printf("📜 %s\n", aws.ToString(policy.PolicyName))
			}
			return nil
		},
	}
}

func displayIAMRolePoliciesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "display-iam-role-policies",
		Short: "Display IAM policies of a role",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return deverrors.NewUsageError("role name is required")
			}
			cfg, err := loadAWSConfig()
			if err != nil {
				return err
			}
			result, err := iam.NewFromConfig(cfg).ListAttachedRolePolicies(context.TODO(), &iam.ListAttachedRolePoliciesInput{
				RoleName: aws.String(args[0]),
			})
			if err != nil {
				return deverrors.NewAPIError("list policies for role %s: %v", args[0], err)
			}
			for _, policy := range result.AttachedPolicies {
				fmt.Printf("📜 %s\n", aws.ToString(policy.PolicyName))
			}
			return nil
		},
	}
}
