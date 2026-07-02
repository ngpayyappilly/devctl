package kubehelper

import (
	"context"
	"devctl/pkg/config"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
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
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = os.ExpandEnv("$HOME/.kube/config")
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}

	return kubernetes.NewForConfig(config)
}

func setContextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-context [context] [namespace]",
		Short: "Switch Kubernetes context and namespace",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			exec.Command("kubectl", "config", "use-context", args[0]).Run()
			exec.Command("kubectl", "config", "set-context", "--current", "--namespace="+args[1]).Run()
		},
	}
}

func restartDeploymentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart [deployment]",
		Short: "Restart a deployment in current K8s namespace",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			exec.Command("kubectl", "rollout", "restart", "deployment/"+args[0]).Run()
		},
	}
}

func getLogsFromPodCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs [pod]",
		Short: "Tail logs from a pod",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			exec.Command("kubectl", "logs", "-f", args[0]).Run()
		},
	}
}

func getPodsCmd() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "get-pods",
		Short: "List pods in a namespace",
		Run: func(cmd *cobra.Command, args []string) {
			if !cmd.Flags().Changed("namespace") {
				namespace = config.GetString(config.KeyKubeNamespace, "default")
			}

			clientset, err := getKubeClient()
			if err != nil {
				log.Fatalf("Failed to create Kubernetes client: %v", err)
			}

			pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				log.Fatalf("Error fetching pods: %v", err)
			}

			for _, pod := range pods.Items {
				fmt.Printf("🟢 %s (%s)\n", pod.Name, pod.Status.Phase)
			}
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace to list pods in (falls back to config, then \"default\")")
	return cmd
}

func currentContextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current-context",
		Short: "Show the current Kubernetes context",
		Run: func(cmd *cobra.Command, args []string) {
			kubeconfig := os.Getenv("KUBECONFIG")
			if kubeconfig == "" {
				kubeconfig = os.ExpandEnv("$HOME/.kube/config")
			}
			config, err := clientcmd.LoadFromFile(kubeconfig)
			if err != nil {
				log.Fatalf("Failed to load kubeconfig: %v", err)
			}

			fmt.Printf("📌 Current context: %s\n", config.CurrentContext)
		},
	}
}
