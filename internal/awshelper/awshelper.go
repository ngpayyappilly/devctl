package awshelper

import (
	"context"
	"devctl/pkg/config"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spf13/cobra"
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
	//CloudFormation commands
	cmd.AddCommand(listStacksCmd())
	cmd.AddCommand(deleteStackCmd())
	cmd.AddCommand(checkStackDriftCmd())
	//IAM commands
	cmd.AddCommand(listIAMUsersCmd())
	cmd.AddCommand(listIAMRolesCmd())
	cmd.AddCommand(listIAMPoliciesCmd())
	cmd.AddCommand(displayIAMRolePoliciesCmd())
	
	return cmd
}

func loadAWSConfig() aws.Config {
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("❌ Failed to load AWS config: %v", err)
	}
	return cfg
}

func listS3Cmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-s3",
		Short: "List all S3 buckets",
		Run: func(cmd *cobra.Command, args []string) {
			cfg := loadAWSConfig()
			client := s3.NewFromConfig(cfg)

			result, err := client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
			if err != nil {
				log.Fatalf("❌ Unable to list buckets: %v", err)
			}

			for _, bucket := range result.Buckets {
				fmt.Printf("🪣 %s\n", aws.ToString(bucket.Name))
			}
		},
	}
}

// list the objects in the bucket
func listBucketObjectsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-bucket-objects",
		Short: "List objects in an S3 bucket",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				log.Fatal("❌ Bucket name is required")
			}
			bucketName := args[0]

			cfg := loadAWSConfig()
			client := s3.NewFromConfig(cfg)

			result, err := client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
				Bucket: aws.String(bucketName),
			})
			if err != nil {
				log.Fatalf("❌ Unable to list objects: %v", err)
			}

			for _, object := range result.Contents {
				fmt.Printf("📦 %s\n", aws.ToString(object.Key))
			}
		},
	}
}

// display bucket policies of a bucket
func displayBucketPolicyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "display-bucket-policy",
		Short: "Display S3 bucket policy",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				log.Fatal("❌ Bucket name is required")
			}
			bucketName := args[0]

			cfg := loadAWSConfig()
			client := s3.NewFromConfig(cfg)

			result, err := client.GetBucketPolicy(context.TODO(), &s3.GetBucketPolicyInput{
				Bucket: aws.String(bucketName),
			})
			if err != nil {
				log.Fatalf("❌ Unable to get bucket policy: %v", err)
			}

			fmt.Printf("🪣 Bucket Policy for %s:\n%s\n", bucketName, aws.ToString(result.Policy))
		},
	}
}

func listEC2Cmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-ec2",
		Short: "List EC2 instances",
		Run: func(cmd *cobra.Command, args []string) {
			cfg := loadAWSConfig()
			client := ec2.NewFromConfig(cfg)

			output, err := client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{})
			if err != nil {
				log.Fatalf("❌ Failed to describe instances: %v", err)
			}

			for _, res := range output.Reservations {
				for _, inst := range res.Instances {
					fmt.Printf("🖥️ Instance ID: %s | State: %s | Type: %s\n",
						aws.ToString(inst.InstanceId),
						string(inst.State.Name),
						string(inst.InstanceType))
				}
			}
		},
	}
}

// display ec2 instance details
func displayEC2DetailsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "display-ec2",
		Short: "Display EC2 instance details",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				log.Fatal("❌ Instance ID is required")
			}
			instanceID := args[0]

			cfg := loadAWSConfig()
			client := ec2.NewFromConfig(cfg)

			output, err := client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{
				InstanceIds: []string{instanceID},
			})
			if err != nil {
				log.Fatalf("❌ Failed to describe instance: %v", err)
			}

			for _, res := range output.Reservations {
				for _, inst := range res.Instances {
					fmt.Printf("🖥️ Instance ID: %s | State: %s | Type: %s\n",
						aws.ToString(inst.InstanceId),
						string(inst.State.Name),
						string(inst.InstanceType))
				}
			}
		},
	}
}

func listStacksCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-cf-stacks",
		Short: "List CloudFormation stacks",
		Run: func(cmd *cobra.Command, args []string) {
			cfg := loadAWSConfig()
			client := cloudformation.NewFromConfig(cfg)

			resp, err := client.ListStacks(context.TODO(), &cloudformation.ListStacksInput{})
			if err != nil {
				log.Fatalf("❌ Failed to list stacks: %v", err)
			}

			for _, summary := range resp.StackSummaries {
				fmt.Printf("🧱 Stack: %s | Status: %s\n",
					aws.ToString(summary.StackName),
					string(summary.StackStatus))
			}
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
		Run: func(cmd *cobra.Command, args []string) {
			if instanceID == "" || keyPath == "" {
				log.Fatal("❌ Instance ID and key path are required")
			}

			// Fall back to config file / env var when flag was not explicitly set.
			if !cmd.Flags().Changed("region") {
				region = config.GetString(config.KeyAWSRegion, "us-east-1")
			}
			if !cmd.Flags().Changed("user") {
				username = config.GetString(config.KeySSHUsername, "ec2-user")
			}

			cfg, err := awsconfig.LoadDefaultConfig(context.TODO(), awsconfig.WithRegion(region))
			if err != nil {
				log.Fatalf("❌ Failed to load AWS config: %v", err)
			}

			client := ec2.NewFromConfig(cfg)
			out, err := client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{
				InstanceIds: []string{instanceID},
			})
			if err != nil || len(out.Reservations) == 0 || len(out.Reservations[0].Instances) == 0 {
				log.Fatalf("❌ Failed to describe instance %s: %v", instanceID, err)
			}

			inst := out.Reservations[0].Instances[0]
			if inst.PublicIpAddress == nil {
				log.Fatal("❌ Instance does not have a public IP")
			}

			ip := aws.ToString(inst.PublicIpAddress)
			sshCmd := fmt.Sprintf("ssh -i %s %s@%s", keyPath, username, ip)
			fmt.Printf("👉 Executing: %s\n", sshCmd)
			if err = executeShell(sshCmd); err != nil {
				log.Fatalf("❌ SSH command failed: %v", err)
			}
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

// executeShell runs a local shell command
func executeShell(command string) error {
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Delete cloudformation stack
func deleteStackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete-cf-stack",
		Short: "Delete a CloudFormation stack",
		Run: func(cmd *cobra.Command, args []string) {
			cfg := loadAWSConfig()
			client := cloudformation.NewFromConfig(cfg)

			if len(args) == 0 {
				log.Fatal("❌ Stack name is required")
			}
			stackName := args[0]

			_, err := client.DeleteStack(context.TODO(), &cloudformation.DeleteStackInput{
				StackName: aws.String(stackName),
			})
			if err != nil {
				log.Fatalf("❌ Failed to delete stack %s: %v", stackName, err)
			}

			fmt.Printf("✅ Stack %s deletion initiated.\n", stackName)
		},
	}
}

// check the cloudformation stack is drifted or not
func checkStackDriftCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check-stack-drift",
		Short: "Check if a CloudFormation stack is drifted",
		Run: func(cmd *cobra.Command, args []string) {
			cfg := loadAWSConfig()
			client := cloudformation.NewFromConfig(cfg)

			if len(args) == 0 {
				log.Fatal("❌ Stack name is required")
			}
			stackName := args[0]

			_, err := client.DescribeStackResourceDrifts(context.TODO(), &cloudformation.DescribeStackResourceDriftsInput{
				StackName: aws.String(stackName),
			})
			if err != nil {
				log.Fatalf("❌ Failed to check stack drift for %s: %v", stackName, err)
			}

			fmt.Printf("✅ Stack %s drift check initiated.\n", stackName)
		},
	}
}

// List IAM Users
func listIAMUsersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-iam-users",
		Short: "List IAM users",
		Run: func(cmd *cobra.Command, args []string) {
			cfg := loadAWSConfig()
			client := iam.NewFromConfig(cfg)

			result, err := client.ListUsers(context.TODO(), &iam.ListUsersInput{})
			if err != nil {
				log.Fatalf("❌ Unable to list IAM users: %v", err)
			}

			for _, user := range result.Users {
				fmt.Printf("👤 %s\n", aws.ToString(user.UserName))
			}
		},
	}
}

// List IAM Roles
func listIAMRolesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-iam-roles",
		Short: "List IAM roles",
		Run: func(cmd *cobra.Command, args []string) {
			cfg := loadAWSConfig()
			client := iam.NewFromConfig(cfg)

			result, err := client.ListRoles(context.TODO(), &iam.ListRolesInput{})
			if err != nil {
				log.Fatalf("❌ Unable to list IAM roles: %v", err)
			}

			for _, role := range result.Roles {
				fmt.Printf("👤 %s\n", aws.ToString(role.RoleName))
			}
		},
	}
}

// List IAM Policies
func listIAMPoliciesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-iam-policies",
		Short: "List IAM policies",
		Run: func(cmd *cobra.Command, args []string) {
			cfg := loadAWSConfig()
			client := iam.NewFromConfig(cfg)

			result, err := client.ListPolicies(context.TODO(), &iam.ListPoliciesInput{})
			if err != nil {
				log.Fatalf("❌ Unable to list IAM policies: %v", err)
			}

			for _, policy := range result.Policies {
				fmt.Printf("📜 %s\n", aws.ToString(policy.PolicyName))
			}
		},
	}
}

// Display IAM Policies of a Role
func displayIAMRolePoliciesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "display-iam-role-policies",
		Short: "Display IAM policies of a role",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				log.Fatal("❌ Role name is required")
			}
			roleName := args[0]

			cfg := loadAWSConfig()
			client := iam.NewFromConfig(cfg)

			result, err := client.ListAttachedRolePolicies(context.TODO(), &iam.ListAttachedRolePoliciesInput{
				RoleName: aws.String(roleName),
			})
			if err != nil {
				log.Fatalf("❌ Unable to list policies for role %s: %v", roleName, err)
			}

			for _, policy := range result.AttachedPolicies {
				fmt.Printf("📜 %s\n", aws.ToString(policy.PolicyName))
			}
		},
	}
}
