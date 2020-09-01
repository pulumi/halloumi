package orchestrator

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/ecr"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/ecs"
	elb "github.com/pulumi/pulumi-aws/sdk/v3/go/aws/elasticloadbalancingv2"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/iam"
	"github.com/pulumi/pulumi-docker/sdk/v2/go/docker"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v2/go/x/auto"
)

func DryRun(programPath string) (map[string]*App, error) {
	drEnv := "DRY_RUN=true"
	args := []string{"run", programPath}
	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(), drEnv)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	cmd.StdoutPipe()
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	apps := make(map[string]*App)
	lines := strings.Split(stdout.String(), "\n")
	for _, l := range lines {
		if strings.HasPrefix(l, "HALLOUMI::app") {
			parts := strings.Split(l, seperator)
			appName := parts[1]
			apps[appName] = &App{
				name: appName,
			}
		} else if strings.HasPrefix(l, "HALLOUMI::webservice") {
			parts := strings.Split(l, seperator)
			appName := parts[1]
			wsName := parts[2]
			app := apps[appName]
			app.services = append(apps[appName].services, WebService{name: wsName, appName: appName})
			apps[appName] = app
		}
	}
	return apps, nil
}

const seperator = "____"

type App struct {
	name     string
	services []WebService
}

type WebService struct {
	appName string
	name    string
}

func Deploy(apps map[string]*App) error {
	for k, a := range apps {
		fmt.Printf("Deployming app %s\n", a.name)
		err := a.Deploy()
		if err != nil {
			return errors.Wrapf(err, "failed to deploy app %s: ", k)
		}
		fmt.Printf("Successfully deployed app %s\n", a.name)
	}
	return nil
}

// TODO right now apps are deployed serially, this means that inter-app webservice dependencies don't work
func (a *App) Deploy() error {
	// setup workspace
	ctx := context.Background()
	ws, err := auto.NewLocalWorkspace(ctx, auto.Program(appPulumiFunc), auto.Project(getProject()))

	//install neccessary plugins
	err = ws.InstallPlugin(ctx, "aws", "v3.1.0")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = ws.InstallPlugin(ctx, "docker", "v2.3.0")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// create stack on behalf of user
	user, err := ws.WhoAmI(ctx)
	if err != nil {
		return err
	}
	fqsn := auto.FullyQualifiedStackName(user, "halloumi", fmt.Sprintf("app%s", a.name))
	s, err := auto.NewStack(ctx, fqsn, ws)
	if err != nil {
		s, err = auto.SelectStack(ctx, fqsn, ws)
		if err != nil {
			return err
		}
	}

	err = s.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: "us-west-2"})
	if err != nil {
		return err
	}

	// refresh to avoid issues with drift
	_, err = s.Refresh(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to refresh: ")
	}
	uRes, err := s.Up(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to update: ")
	}
	var subnetIDs []string
	sids, ok := uRes.Outputs["subnetIDs"].Value.([]interface{})
	if !ok {
		return errors.New("failed to unmarshal subnetIds")
	}
	for _, sid := range sids {
		subnetIDs = append(subnetIDs, sid.(string))
	}
	sgID, ok := uRes.Outputs["sgID"].Value.(string)
	if !ok {
		return errors.New("failed to unmarshal sgid")
	}
	vpcID, ok := uRes.Outputs["vpcID"].Value.(string)
	if !ok {
		return errors.New("failed to unmarshal vpcid")
	}
	taskExecRoleArn, ok := uRes.Outputs["taskExecRoleArn"].Value.(string)
	if !ok {
		return errors.New("failed to unmarshal taskExecRoleArn")
	}
	imageName, ok := uRes.Outputs["imageName"].Value.(string)
	if !ok {
		return errors.New("failed to unmarshal imageName")
	}
	clusterArn, ok := uRes.Outputs["clusterArn"].Value.(string)
	if !ok {
		return errors.New("failed to unmarshal clusterArn")
	}

	var envs []envVar

	wsi := wsInput{
		subnetIDs,
		sgID,
		vpcID,
		taskExecRoleArn,
		imageName,
		clusterArn,
		envs,
	}

	return a.DeployServices(wsi)
}

func (a *App) DeployServices(wsi wsInput) error {
	// TODO: accumulate environement vairables as we deploy these stacks
	var urlEnvs []envVar

	for _, s := range a.services {
		fmt.Printf("Deploying service %s\n", s.name)
		runEnv := envVar{
			Name:  fmt.Sprintf("HALLOUMI_%s_%s", a.name, s.name),
			Value: "1",
		}
		wsi.envs = append(urlEnvs, runEnv)
		pFunc := getPulumiServiceFunc(wsi)

		ctx := context.Background()
		ws, err := auto.NewLocalWorkspace(ctx, auto.Program(pFunc), auto.Project(getProject()))

		// create stack on behalf of user
		user, err := ws.WhoAmI(ctx)
		if err != nil {
			return err
		}
		fqsn := auto.FullyQualifiedStackName(user, "halloumi", fmt.Sprintf("app%ssvc%s", a.name, s.name))
		stack, err := auto.NewStack(ctx, fqsn, ws)
		if err != nil {
			stack, err = auto.SelectStack(ctx, fqsn, ws)
			if err != nil {
				return err
			}
		}

		err = stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: "us-west-2"})
		if err != nil {
			return err
		}

		// refresh to avoid issues with drift
		_, err = stack.Refresh(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to refresh: ")
		}
		uRes, err := stack.Up(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to update: ")
		}
		url := uRes.Outputs["url"].Value.(string)
		urlEnvs = append(urlEnvs, envVar{
			Name:  fmt.Sprintf("HALLOUMI_%s_URL", s.name),
			Value: url,
		})
		fmt.Printf("Successfully deployed service %s\n", s.name)
		fmt.Printf("Service %s running at: %s\n", s.name, url)
	}

	return nil
}

type wsInput struct {
	subnetIDs       []string
	sgID            string
	vpcID           string
	taskExecRoleArn string
	imageName       string
	clusterArn      string
	envs            []envVar
}

type envVar struct {
	Name  string
	Value string
}

var appPulumiFunc = func(ctx *pulumi.Context) error {
	// Read back the default VPC and public subnets, which we will use.
	t := true
	vpc, err := ec2.LookupVpc(ctx, &ec2.LookupVpcArgs{Default: &t})
	if err != nil {
		return err
	}
	subnet, err := ec2.GetSubnetIds(ctx, &ec2.GetSubnetIdsArgs{VpcId: vpc.Id})
	if err != nil {
		return err
	}

	// Create a SecurityGroup that permits HTTP ingress and unrestricted egress.
	webSg, err := ec2.NewSecurityGroup(ctx, "web-sg", &ec2.SecurityGroupArgs{
		VpcId: pulumi.String(vpc.Id),
		Egress: ec2.SecurityGroupEgressArray{
			ec2.SecurityGroupEgressArgs{
				Protocol:   pulumi.String("-1"),
				FromPort:   pulumi.Int(0),
				ToPort:     pulumi.Int(0),
				CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
			},
		},
		Ingress: ec2.SecurityGroupIngressArray{
			ec2.SecurityGroupIngressArgs{
				Protocol:   pulumi.String("tcp"),
				FromPort:   pulumi.Int(80),
				ToPort:     pulumi.Int(80),
				CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
			},
		},
	})
	if err != nil {
		return err
	}

	// Create an ECS cluster to run a container-based service.
	cluster, err := ecs.NewCluster(ctx, "app-cluster", nil)
	if err != nil {
		return err
	}

	// Create an IAM role that can be used by our service's task.
	taskExecRole, err := iam.NewRole(ctx, "task-exec-role", &iam.RoleArgs{
		AssumeRolePolicy: pulumi.String(`{
"Version": "2008-10-17",
"Statement": [{
"Sid": "",
"Effect": "Allow",
"Principal": {
	"Service": "ecs-tasks.amazonaws.com"
},
"Action": "sts:AssumeRole"
}]
}`),
	})
	if err != nil {
		return err
	}
	_, err = iam.NewRolePolicyAttachment(ctx, "task-exec-policy", &iam.RolePolicyAttachmentArgs{
		Role:      taskExecRole.Name,
		PolicyArn: pulumi.String("arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"),
	})
	if err != nil {
		return err
	}

	repo, err := ecr.NewRepository(ctx, "foo", &ecr.RepositoryArgs{})
	if err != nil {
		return err
	}

	repoCreds := repo.RegistryId.ApplyStringArray(func(rid string) ([]string, error) {
		creds, err := ecr.GetCredentials(ctx, &ecr.GetCredentialsArgs{
			RegistryId: rid,
		})
		if err != nil {
			return nil, err
		}
		data, err := base64.StdEncoding.DecodeString(creds.AuthorizationToken)
		if err != nil {
			fmt.Println("error:", err)
			return nil, err
		}

		return strings.Split(string(data), ":"), nil
	})
	repoUser := repoCreds.Index(pulumi.Int(0))
	repoPass := repoCreds.Index(pulumi.Int(1))

	image, err := docker.NewImage(ctx, "my-image", &docker.ImageArgs{
		Build: docker.DockerBuildArgs{
			Context: pulumi.String(filepath.Join(".")),
		},
		ImageName: pulumi.Sprintf("%s:%d", repo.RepositoryUrl, pulumi.Int(time.Now().Unix())),
		Registry: docker.ImageRegistryArgs{
			Server:   repo.RepositoryUrl,
			Username: repoUser,
			Password: repoPass,
		},
	})

	ctx.Export("subnetIDs", toPulumiStringArray(subnet.Ids))
	ctx.Export("sgID", webSg.ID().ToStringOutput())
	ctx.Export("vpcID", pulumi.String(vpc.Id))
	ctx.Export("taskExecRoleArn", taskExecRole.Arn)
	ctx.Export("imageName", image.ImageName)
	ctx.Export("clusterArn", cluster.Arn)

	return nil
}

// TODO - this doesn't seem to be updating as I would expect
func getPulumiServiceFunc(wsi wsInput) pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		// Create a load balancer to listen for HTTP traffic on port 80.
		webLb, err := elb.NewLoadBalancer(ctx, "web-lb", &elb.LoadBalancerArgs{
			Subnets:        toPulumiStringArray(wsi.subnetIDs),
			SecurityGroups: pulumi.StringArray{pulumi.String(wsi.sgID)},
		})
		if err != nil {
			return err
		}
		webTg, err := elb.NewTargetGroup(ctx, "web-tg", &elb.TargetGroupArgs{
			Port:       pulumi.Int(80),
			Protocol:   pulumi.String("HTTP"),
			TargetType: pulumi.String("ip"),
			VpcId:      pulumi.String(wsi.vpcID),
		})
		if err != nil {
			return err
		}
		webListener, err := elb.NewListener(ctx, "web-listener", &elb.ListenerArgs{
			LoadBalancerArn: webLb.Arn,
			Port:            pulumi.Int(80),
			DefaultActions: elb.ListenerDefaultActionArray{
				elb.ListenerDefaultActionArgs{
					Type:           pulumi.String("forward"),
					TargetGroupArn: webTg.Arn,
				},
			},
		})
		if err != nil {
			return err
		}

		fmtstr := `[{
			"name": "my-app",
			"image": %q,
			"environment": %s,
			"portMappings": [{
				"containerPort": 80,
				"hostPort": 80,
				"protocol": "tcp"
			}]
		}]`

		envStr, err := json.Marshal(wsi.envs)
		if err != nil {
			return err
		}

		containerDef := pulumi.String(fmt.Sprintf(fmtstr, wsi.imageName, envStr))

		// Spin up a load balanced service running NGINX.
		appTask, err := ecs.NewTaskDefinition(ctx, "app-task", &ecs.TaskDefinitionArgs{
			Family:                  pulumi.String("fargate-task-definition"),
			Cpu:                     pulumi.String("256"),
			Memory:                  pulumi.String("512"),
			NetworkMode:             pulumi.String("awsvpc"),
			RequiresCompatibilities: pulumi.StringArray{pulumi.String("FARGATE")},
			ExecutionRoleArn:        pulumi.String(wsi.taskExecRoleArn),
			ContainerDefinitions:    pulumi.String(containerDef),
		})
		if err != nil {
			return err
		}
		_, err = ecs.NewService(ctx, "app-svc", &ecs.ServiceArgs{
			Cluster:        pulumi.String(wsi.clusterArn),
			DesiredCount:   pulumi.Int(5),
			LaunchType:     pulumi.String("FARGATE"),
			TaskDefinition: appTask.Arn,
			NetworkConfiguration: &ecs.ServiceNetworkConfigurationArgs{
				AssignPublicIp: pulumi.Bool(true),
				Subnets:        toPulumiStringArray(wsi.subnetIDs),
				SecurityGroups: pulumi.StringArray{pulumi.String(wsi.sgID)},
			},
			LoadBalancers: ecs.ServiceLoadBalancerArray{
				ecs.ServiceLoadBalancerArgs{
					TargetGroupArn: webTg.Arn,
					ContainerName:  pulumi.String("my-app"),
					ContainerPort:  pulumi.Int(80),
				},
			},
		}, pulumi.DependsOn([]pulumi.Resource{webListener}))

		ctx.Export("url", webLb.DnsName)

		return nil
	}
}

func toPulumiStringArray(a []string) pulumi.StringArrayInput {
	var res []pulumi.StringInput
	for _, s := range a {
		res = append(res, pulumi.String(s))
	}
	return pulumi.StringArray(res)
}

func getProject() workspace.Project {
	proj := workspace.Project{
		Name:    tokens.PackageName("halloumi"),
		Runtime: workspace.NewProjectRuntimeInfo("go", nil),
	}

	return proj
}
