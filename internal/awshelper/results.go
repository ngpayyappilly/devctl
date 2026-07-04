package awshelper

// Bucket represents an S3 bucket.
type Bucket struct {
	Name string `json:"name" yaml:"name"`
}

// BucketList is a slice of Bucket that satisfies output.Tabler.
type BucketList []Bucket

func (bl BucketList) Headers() []string { return []string{"NAME"} }
func (bl BucketList) Rows() [][]string {
	rows := make([][]string, len(bl))
	for i, b := range bl {
		rows[i] = []string{b.Name}
	}
	return rows
}

// Object represents an S3 object key.
type Object struct {
	Key string `json:"key" yaml:"key"`
}

// ObjectList is a slice of Object.
type ObjectList []Object

func (ol ObjectList) Headers() []string { return []string{"KEY"} }
func (ol ObjectList) Rows() [][]string {
	rows := make([][]string, len(ol))
	for i, o := range ol {
		rows[i] = []string{o.Key}
	}
	return rows
}

// Instance represents an EC2 instance summary.
type Instance struct {
	ID    string `json:"id" yaml:"id"`
	State string `json:"state" yaml:"state"`
	Type  string `json:"type" yaml:"type"`
}

// InstanceList is a slice of Instance.
type InstanceList []Instance

func (il InstanceList) Headers() []string { return []string{"ID", "STATE", "TYPE"} }
func (il InstanceList) Rows() [][]string {
	rows := make([][]string, len(il))
	for i, inst := range il {
		rows[i] = []string{inst.ID, inst.State, inst.Type}
	}
	return rows
}

// Stack represents a CloudFormation stack.
type Stack struct {
	Name   string `json:"name" yaml:"name"`
	Status string `json:"status" yaml:"status"`
}

// StackList is a slice of Stack.
type StackList []Stack

func (sl StackList) Headers() []string { return []string{"NAME", "STATUS"} }
func (sl StackList) Rows() [][]string {
	rows := make([][]string, len(sl))
	for i, s := range sl {
		rows[i] = []string{s.Name, s.Status}
	}
	return rows
}

// IAMUser represents an IAM user.
type IAMUser struct {
	UserName string `json:"username" yaml:"username"`
}

// IAMUserList is a slice of IAMUser.
type IAMUserList []IAMUser

func (ul IAMUserList) Headers() []string { return []string{"USERNAME"} }
func (ul IAMUserList) Rows() [][]string {
	rows := make([][]string, len(ul))
	for i, u := range ul {
		rows[i] = []string{u.UserName}
	}
	return rows
}

// IAMRole represents an IAM role.
type IAMRole struct {
	RoleName string `json:"role_name" yaml:"role_name"`
}

// IAMRoleList is a slice of IAMRole.
type IAMRoleList []IAMRole

func (rl IAMRoleList) Headers() []string { return []string{"ROLE NAME"} }
func (rl IAMRoleList) Rows() [][]string {
	rows := make([][]string, len(rl))
	for i, r := range rl {
		rows[i] = []string{r.RoleName}
	}
	return rows
}

// IAMPolicy represents an IAM policy name.
type IAMPolicy struct {
	PolicyName string `json:"policy_name" yaml:"policy_name"`
}

// IAMPolicyList is a slice of IAMPolicy.
type IAMPolicyList []IAMPolicy

func (pl IAMPolicyList) Headers() []string { return []string{"POLICY NAME"} }
func (pl IAMPolicyList) Rows() [][]string {
	rows := make([][]string, len(pl))
	for i, p := range pl {
		rows[i] = []string{p.PolicyName}
	}
	return rows
}
