package kubehelper

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"devctl/pkg/config"
	"devctl/pkg/output"
)

func NewKubeHelperCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kube",
		Short: "Perform quick actions with Kubernetes",
	}

	cmd.AddCommand(getPodsCmd())
	cmd.AddCommand(currentContextCmd())
	cmd.AddCommand(setContextCmd())
	cmd.AddCommand(restartDeploymentCmd())
	cmd.AddCommand(getLogsFromPodCmd())

	return cmd
}

func getKubeClient() (*kubernetes.Clientset, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = os.ExpandEnv("$HOME/.kube/config")
		}
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}
	return kubernetes.NewForConfig(cfg)
}

func setContextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-context [context] [namespace]",
		Short: "Switch Kubernetes context and namespace",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := exec.Command("kubectl", "config", "use-context", args[0]).Run(); err != nil {
				return fmt.Errorf("set context %s: %w", args[0], err)
			}
			if err := exec.Command("kubectl", "config", "set-context", "--current", "--namespace="+args[1]).Run(); err != nil {
				return fmt.Errorf("set namespace %s: %w", args[1], err)
			}
			return nil
		},
	}
}

func restartDeploymentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart [deployment]",
		Short: "Restart a deployment in current K8s namespace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] Would run: kubectl rollout restart deployment/%s\n", args[0])
				return nil
			}
			if err := exec.Command("kubectl", "rollout", "restart", "deployment/"+args[0]).Run(); err != nil {
				return fmt.Errorf("restart deployment %s: %w", args[0], err)
			}
			return nil
		},
	}
}

func getLogsFromPodCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs [pod]",
		Short: "Tail logs from a pod",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := exec.Command("kubectl", "logs", "-f", args[0]).Run(); err != nil {
				return fmt.Errorf("fetch logs for pod %s: %w", args[0], err)
			}
			return nil
		},
	}
}

func getPodsCmd() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "get-pods",
		Short: "List pods in a namespace",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("namespace") {
				namespace = config.GetString(config.KeyKubeNamespace, "")
			}

			clientset, err := getKubeClient()
			if err != nil {
				return fmt.Errorf("create Kubernetes client: %w", err)
			}

			pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				return fmt.Errorf("list pods in namespace %q: %w", namespace, err)
			}

			podList := make(PodList, len(pods.Items))
			for i, pod := range pods.Items {
				podList[i] = Pod{Name: pod.Name, Status: string(pod.Status.Phase)}
			}
			return output.New(output.FormatFromCmd(cmd)).Print(cmd.OutOrStdout(), podList)
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace to list pods in (falls back to config, then \"default\")")
	return cmd
}

func currentContextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current-context",
		Short: "Show the current Kubernetes context",
		RunE: func(cmd *cobra.Command, args []string) error {
			kubeconfig := os.Getenv("KUBECONFIG")
			if kubeconfig == "" {
				kubeconfig = os.ExpandEnv("$HOME/.kube/config")
			}
			cfg, err := clientcmd.LoadFromFile(kubeconfig)
			if err != nil {
				return fmt.Errorf("load kubeconfig: %w", err)
			}

			return output.New(output.FormatFromCmd(cmd)).Print(cmd.OutOrStdout(), ContextResult{Context: cfg.CurrentContext})
		},
	}
}
