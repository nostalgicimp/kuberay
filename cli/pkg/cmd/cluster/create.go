package cluster

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ray-project/kuberay/cli/pkg/cmdutil"
	"github.com/ray-project/kuberay/proto/go_client"
	"github.com/spf13/cobra"
)

type CreateOptions struct {
	name                  string
	namespace             string
	environment           string
	version               string
	user                  string
	headComputeTemplate   string
	headImage             string
	headServiceType       string
	workerGroupName       string
	workerComputeTemplate string
	workerImage           string
	workerReplicas        uint32
}

func newCmdCreate() *cobra.Command {
	opts := CreateOptions{}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a ray cluster",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return createCluster(opts)
		},
	}

	cmd.Flags().StringVar(&opts.name, "name", "", "name of the cluster")
	cmd.Flags().StringVar(&opts.namespace, "namespace", "ray-system",
		"kubernetes namespace where the cluster will be")
	cmd.Flags().StringVar(&opts.environment, "environment", "DEV",
		"environment of the cluster (valid values: DEV, TESTING, STAGING, PRODUCTION)")
	cmd.Flags().StringVar(&opts.version, "version", "1.9.0", "version of the ray cluster")
	cmd.Flags().StringVar(&opts.user, "user", "", "SSO username of ray cluster creator")
	cmd.Flags().StringVar(&opts.headComputeTemplate, "head-compute-tempalte", "", "compuate template name for ray head")
	cmd.Flags().StringVar(&opts.headImage, "head-image", "", "ray head image")
	cmd.Flags().StringVar(&opts.headServiceType, "head-service-type", "ClusterIP", "ray head service type (ClusterIP, NodePort, LoadBalancer)")
	cmd.Flags().StringVar(&opts.workerGroupName, "worker-group-name", "", "first worker group name")
	cmd.Flags().StringVar(&opts.workerComputeTemplate, "worker-compute-template", "", "compute template name of worker in the first worker group")
	cmd.Flags().StringVar(&opts.workerImage, "worker-image", "", "image of worker in the first worker group")
	cmd.Flags().Uint32Var(&opts.workerReplicas, "worker-replicas", 1, "pod replicas of workers in the first worker group")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("user")
	cmd.MarkFlagRequired("head-image")
	cmd.MarkFlagRequired("head-compute-tempalte")
	cmd.MarkFlagRequired("worker-image")
	cmd.MarkFlagRequired("worker-compute-template")

	// handle user from auth and inject it.

	return cmd
}

func createCluster(opts CreateOptions) error {
	conn, err := cmdutil.GetGrpcConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	// build gRPC client
	client := go_client.NewClusterServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	envInt, ok := go_client.Cluster_Environment_value[opts.environment]
	if !ok {
		fmt.Fprintf(os.Stderr, "error: Invalid environment value. Valid values: DEV, TESTING, STAGING, PRODUCTION\n")
		os.Exit(1)
	}

	headStartParams := make(map[string]string)
	headStartParams["port"] = "6379"
	headStartParams["dashboard-host"] = "0.0.0.0"
	headStartParams["node-ip-address"] = "$MY_POD_IP"
	headStartParams["redis-password"] = "LetMeInRay"

	headSpec := &go_client.HeadGroupSpec{
		ComputeTemplate: opts.headComputeTemplate,
		Image:           opts.headImage,
		ServiceType:     opts.headServiceType,
		RayStartParams:  headStartParams,
	}

	workerStartParams := make(map[string]string)
	workerStartParams["node-ip-address"] = "$MY_POD_IP"
	workerStartParams["redis-password"] = "LetMeInRay"

	var workerGroupSpecs []*go_client.WorkerGroupSpec
	spec := &go_client.WorkerGroupSpec{
		GroupName:       opts.workerGroupName,
		ComputeTemplate: opts.workerComputeTemplate,
		Image:           opts.workerImage,
		Replicas:        int32(opts.workerReplicas),
		MinReplicas:     int32(opts.workerReplicas),
		MaxReplicas:     int32(opts.workerReplicas),
		RayStartParams:  workerStartParams,
	}
	workerGroupSpecs = append(workerGroupSpecs, spec)

	cluster := &go_client.Cluster{
		Name:        opts.name,
		Namespace:   opts.namespace,
		User:        opts.user,
		Version:     opts.version,
		Environment: *go_client.Cluster_Environment(envInt).Enum(),
		ClusterSpec: &go_client.ClusterSpec{
			HeadGroupSpec:   headSpec,
			WorkerGroupSepc: workerGroupSpecs,
		},
	}

	r, err := client.CreateCluster(ctx, &go_client.CreateClusterRequest{
		Cluster: cluster,
	})
	if err != nil {
		log.Fatalf("could not create cluster %v", err)
	}

	log.Printf("cluster %v is created", r.Name)
	return nil
}
