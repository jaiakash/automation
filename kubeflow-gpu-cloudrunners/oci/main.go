package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cncf/automation/cloudrunners/oci/pkg/oci"
	"github.com/cncf/automation/cloudrunners/pkg/remote"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

var Cmd = &cobra.Command{
	Use:  "gha-gpu-runner",
	Long: "Run a GitHub Actions runner (on GPU powered Oracle Cloud Infrastructure)",
	RunE: run,
}

var args struct {
	debug bool

	arch                string
	compartmentId       string
	subnetId            string
	availabilityDomain  string
	shape               string
	bootVolumeSizeInGBs int64
	imageId             string
}

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)

	if err := Cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	os.Exit(0)
}

func run(cmd *cobra.Command, argv []string) error {
	ctx := context.Background()

	// Initialize the OCI clients
	computeClient, err := core.NewComputeClientWithConfigurationProvider(common.DefaultConfigProvider())
	if err != nil {
		return fmt.Errorf("failed to create compute client: %w", err)
	}
	networkClient, err := core.NewVirtualNetworkClientWithConfigurationProvider(common.DefaultConfigProvider())
	if err != nil {
		return fmt.Errorf("failed to create network client: %w", err)
	}

	if args.imageId == "" {
		return fmt.Errorf("must provide --image-id for the instance")
	}

	// Create SSH Key Pair
	sshKeyPair, err := remote.CreateSSHKeyPair()
	if err != nil {
		return fmt.Errorf("creating ssh key pair: %w", err)
	}

	launchDetails := core.LaunchInstanceDetails{
		CompartmentId:      common.String(args.compartmentId),
		AvailabilityDomain: common.String(args.availabilityDomain),
		Shape:              common.String(args.shape),
		ImageId:            common.String(args.imageId),
		DisplayName:        common.String(fmt.Sprintf("kubeflow-gha-gpu-runner-%s-%s", args.arch, time.Now().Format("20060102-150405"))),
		CreateVnicDetails: &core.CreateVnicDetails{
			AssignPublicIp: common.Bool(true),
			SubnetId:       common.String(args.subnetId),
		},
		Metadata: map[string]string{
			"ssh_authorized_keys": sshKeyPair.PublicKey,
		},
	}

	machine, err := oci.NewEphemeralMachine(ctx, computeClient, networkClient, launchDetails)
	if err != nil {
		return fmt.Errorf("failed to create machine: %w", err)
	}

	defer func() {
		err := machine.Delete(context.Background())
		if err != nil {
			log.Printf("failed to delete machine: %v", err)
		}
	}()

	// Wait for the machine to be ready
	time.Sleep(30 * time.Second)
	err = machine.WaitForInstanceReady(ctx)
	if err != nil {
		return fmt.Errorf("failed to wait for instance to be ready: %w", err)
	}

	ip := machine.ExternalIP()
	if ip == "" {
		return fmt.Errorf("cannot find ip for instance")
	}

	sshConfig := &ssh.ClientConfig{
		User: "ubuntu",
		Auth: []ssh.AuthMethod{
			sshKeyPair.SSHAuth,
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	sshClient, err := remote.DialWithRetry(ctx, "tcp", ip+":22", sshConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to ssh on %q: %w", ip, err)
	}
	defer sshClient.Close()

	commands := []string{
		"tar -zxf /opt/runner-cache/actions-runner-linux-*.tar.gz",
		"rm -rf \\$HOME",
		"sudo chown -R 1000:1000 /etc/skel/",
		"mv /etc/skel/.cargo /home/ubuntu/",
		"mv /etc/skel/.nvm /home/ubuntu/",
		"mv /etc/skel/.rustup /home/ubuntu/",
		"mv /etc/skel/.dotnet /home/ubuntu/",
		"mv /etc/skel/.composer /home/ubuntu/",
		"sudo setfacl -m u:ubuntu:rw /var/run/docker.sock",
		"sudo sysctl fs.inotify.max_user_instances=1280",
		"sudo sysctl fs.inotify.max_user_watches=655360",
		"export PATH=$PATH:/home/ubuntu/.local/bin && export HOME=/home/ubuntu && export NVM_DIR=/home/ubuntu/.nvm && bash -x /home/ubuntu/run.sh --jitconfig \"${ACTIONS_RUNNER_INPUT_JITCONFIG}\"",
	}

	for _, cmd := range commands {
		log.Println("running ssh command", "command", cmd)
		expanded := strings.ReplaceAll(cmd, "${ACTIONS_RUNNER_INPUT_JITCONFIG}", os.Getenv("ACTIONS_RUNNER_INPUT_JITCONFIG"))
		output, err := sshClient.RunCommand(ctx, expanded)
		if err != nil {
			log.Println(err, "running ssh command", "command", cmd, "output", string(output[:]))
			return fmt.Errorf("running command %q: %w", cmd, err)
		}
		log.Println("command succeeded", "command", cmd, "output", string(output))
	}

	return nil
}

func init() {
	flags := Cmd.Flags()

	flags.BoolVar(
		&args.debug,
		"debug",
		true,
		"Enable debug logging")

	flags.StringVar(
		&args.arch,
		"arch",
		"x86",
		"Machine architecture")

	flags.StringVar(
		&args.availabilityDomain,
		"availability-domain",
		"tdbQ:US-ASHBURN-AD-1",
		"Availability Domain")

	flags.StringVar(
		&args.compartmentId,
		"compartment-id",
		"ocid1.compartment.oc1..aaaaaaaaczejzfg7ixiqrl7r4jr5dohrtxfpuhdinrq4okj67hskmhgglyfq",
		"Compartment ID")

	// TODO generic subnet selection based on AD
	flags.StringVar(
		&args.subnetId,
		"subnet-id",
		"ocid1.subnet.oc1.iad.aaaaaaaaff7mqwjlremjpiq72i2wjgfjfjz2dhymvtdybhu5mdaikovb67ka",
		"Subnet ID")

	flags.StringVar(
		&args.shape,
		"shape",
		"VM.GPU.A10.1",
		"VM Shape")

	flags.Int64Var(
		&args.bootVolumeSizeInGBs,
		"boot-volume-size-in-gbs",
		400,
		"Boot volume size in GBs")

    // TODO Setup a custom image with NVIDIA drivers pre-installed
	flags.StringVar(
		&args.imageId,
		"image-id",
		"ocid1.image.oc1.iad.aaaaaaaawkg4mcnr72dcgtprfpjovsvipdabun2xfli7ns3ni7vdn4m3id3a",
		"OCI Image OCID to use for the runner (using GPU based custom image)")
}
