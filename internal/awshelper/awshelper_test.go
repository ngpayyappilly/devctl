package awshelper

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	deverrors "devctl/pkg/errors"
)

// TestListS3Cmd_PropagatesAPIError verifies that when the AWS SDK returns an
// error (no valid credentials/region in CI), the command surfaces an APIError
// rather than calling log.Fatalf and killing the process.
func TestListS3Cmd_PropagatesAPIError(t *testing.T) {
	cmd := listS3Cmd()
	err := cmd.RunE(cmd, nil)

	require.Error(t, err)
	var apiErr *deverrors.APIError
	assert.True(t, errors.As(err, &apiErr), "expected APIError, got %T: %v", err, err)
}

// TestListBucketObjectsCmd_UsageErrorWhenNoBucket verifies that omitting the
// bucket name returns a UsageError (exit code 2) instead of panicking.
func TestListBucketObjectsCmd_UsageErrorWhenNoBucket(t *testing.T) {
	cmd := listBucketObjectsCmd()
	err := cmd.RunE(cmd, nil) // no args

	require.Error(t, err)
	var usageErr *deverrors.UsageError
	assert.True(t, errors.As(err, &usageErr), "expected UsageError, got %T: %v", err, err)
}

// TestDeleteStackCmd_UsageErrorWhenNoStack verifies that omitting the stack
// name returns a UsageError.
func TestDeleteStackCmd_UsageErrorWhenNoStack(t *testing.T) {
	cmd := deleteStackCmd()
	err := cmd.RunE(cmd, nil)

	require.Error(t, err)
	var usageErr *deverrors.UsageError
	assert.True(t, errors.As(err, &usageErr), "expected UsageError, got %T: %v", err, err)
}

// TestDisplayIAMRolePoliciesCmd_UsageErrorWhenNoRole verifies that omitting
// the role name returns a UsageError.
func TestDisplayIAMRolePoliciesCmd_UsageErrorWhenNoRole(t *testing.T) {
	cmd := displayIAMRolePoliciesCmd()
	err := cmd.RunE(cmd, nil)

	require.Error(t, err)
	var usageErr *deverrors.UsageError
	assert.True(t, errors.As(err, &usageErr), "expected UsageError, got %T: %v", err, err)
}
