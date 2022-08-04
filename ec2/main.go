package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Create an AWS resource (S3 Bucket)
		bucket, err := s3.NewBucket(ctx, "my-s3-bucket-pulumi", nil)
		if err != nil {
			return err
		}

		// Export the name of the bucketß
		ctx.Export("bucketName", bucket.ID())
		//return nil
		myVpc, err := ec2.NewVpc(ctx, "myVpc", &ec2.VpcArgs{
			CidrBlock: pulumi.String("172.16.0.0/16"),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("tf-example"),
			},
		})
		if err != nil {
			return err
		}
		mySubnet, err := ec2.NewSubnet(ctx, "mySubnet", &ec2.SubnetArgs{
			VpcId:            myVpc.ID(),
			CidrBlock:        pulumi.String("172.16.10.0/24"),
			AvailabilityZone: pulumi.String("us-west-2a"),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("tf-example"),
			},
		})
		if err != nil {
			return err
		}
		_, err = ec2.NewDefaultSecurityGroup(ctx, "default", &ec2.DefaultSecurityGroupArgs{
			VpcId: myVpc.ID(),
			Ingress: ec2.DefaultSecurityGroupIngressArray{
				&ec2.DefaultSecurityGroupIngressArgs{
					Protocol:   pulumi.String("TCP"),
					CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
					FromPort:   pulumi.Int(22),
					ToPort:     pulumi.Int(22),
				},
				&ec2.DefaultSecurityGroupIngressArgs{
					Protocol: pulumi.String("TCP"),
					Self:     pulumi.Bool(true),
					FromPort: pulumi.Int(22),
					ToPort:   pulumi.Int(22),
				},
			},
			Egress: ec2.DefaultSecurityGroupEgressArray{
				&ec2.DefaultSecurityGroupEgressArgs{
					Protocol:   pulumi.String("TCP"),
					CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
					FromPort:   pulumi.Int(0),
					ToPort:     pulumi.Int(65535),
				},
			},
		})
		if err != nil {
			return err
		}

		nodes := []string{"lead", "node1", "node2"}
		for i, j := range nodes {
			LUD := pulumi.String(`#!/bin/bash
							echo "Hello, Lead!"
							hostnamectl set-hostname master-node
							#######################
# === All Systems === #
#######################
# Ensure system is fully patched
sudo yum -y makecache fast
sudo yum -y update

# Disable swap
sudo swapoff -a

# comment out swap mount in /etc/fstab
#sudo vi /etc/fstab

# Disable default iptables configuration as it will break kubernetes services (API, coredns, etc...)
sudo sh -c "cp /etc/sysconfig/iptables /etc/sysconfig/iptables.ORIG && iptables --flush && iptables --flush && iptables-save > /etc/sysconfig/iptables"
sudo systemctl restart iptables.service

# Load/Enable br_netfilter kernel module and make persistent
sudo modprobe br_netfilter
sudo sh -c "echo '1' > /proc/sys/net/bridge/bridge-nf-call-iptables"
sudo sh -c "echo '1' > /proc/sys/net/bridge/bridge-nf-call-ip6tables"
sudo sh -c "echo 'net.bridge.bridge-nf-call-iptables=1' >> /etc/sysctl.conf"
sudo sh -c "echo 'net.bridge.bridge-nf-call-ip6tables=1' >> /etc/sysctl.conf"

# Install dependencies for docker-ce
sudo yum -y install yum-utils device-mapper-persistent-data lvm2

# Add the docker-ce repository
sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo

# Add the Kubernetes Repository
sudo sh -c 'cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://packages.cloud.google.com/yum/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
       https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOF'

# Update yum cache after adding repository
sudo yum -y makecache fast

# Install latest supported docker runtime (18.06 is the latest runtime supported by Kubernetes v1.13.2)
sudo yum -y install docker-ce-18.06.1.ce

# Install Kubernetes
sudo yum -y install kubelet kubeadm kubectl

# Enable kubectl bash-completion
sudo yum -y install bash-completion
source <(kubectl completion bash)
echo "source <(kubectl completion bash)" >> ~/.bashrc

# Enable docker and kubelet services
sudo systemctl enable docker.service
sudo systemctl enable kubelet.service

# Check what cgroup driver that docker is using
sudo docker info | grep -i cgroup

# Add the cgroup driver from the previous step to the kublet config as an extra argument
sudo sed -i "s/^\(KUBELET_EXTRA_ARGS=\)\(.*\)$/\1\"--cgroup-driver=$(sudo docker info | grep -i cgroup | cut -d" " -f3)\2\"/" /etc/sysconfig/kubelet


#######################
# === Master Only === #
#######################
# Initialize the Kubernetes master using the public IP address of the master as the apiserver-advertise-address. Set the pod-network-cidr to the cidr address used in the network overlay (flannel, weave, etc...) configuration.
curl -s https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml | grep -E '"Network": "[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\/[0-9]{1,2}"' | cut -d'"' -f4
sudo kubeadm init --apiserver-advertise-address=${master_ip_address} --pod-network-cidr=${NETWORK_OVERLAY_CIDR_NET}

# Copy the cluster configuration to the regular users home directory
mkdir -p $HOME/.kube
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config

# Deploy the Flannel Network Overlay
kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml

# check the readiness of nodes
kubectl get nodes

# check that coredns, apiserver, etcd, and flannel pods are running
kubectl get pods --all-namespaces

# List k8s bootstrap tokens
sudo kubeadm token list

# Generate a new k8s bootstrap token !ONLY! if all other tokens have expired
sudo kubeadm token create

# Decode the Discovery Token CA Cert Hash
openssl x509 -pubkey -in /etc/kubernetes/pki/ca.crt | openssl rsa -pubin -outform der 2>/dev/null | openssl dgst -sha256 -hex | sed 's/^.* //'



							`)
			NodeUD := pulumi.String(`#!/bin/bash
							echo "Hello, Node!" 
							#######################
# === All Systems === #
#######################
# Ensure system is fully patched
sudo yum -y makecache fast
sudo yum -y update

# Disable swap
sudo swapoff -a

# comment out swap mount in /etc/fstab
#sudo vi /etc/fstab

# Disable default iptables configuration as it will break kubernetes services (API, coredns, etc...)
sudo sh -c "cp /etc/sysconfig/iptables /etc/sysconfig/iptables.ORIG && iptables --flush && iptables --flush && iptables-save > /etc/sysconfig/iptables"
sudo systemctl restart iptables.service

# Load/Enable br_netfilter kernel module and make persistent
sudo modprobe br_netfilter
sudo sh -c "echo '1' > /proc/sys/net/bridge/bridge-nf-call-iptables"
sudo sh -c "echo '1' > /proc/sys/net/bridge/bridge-nf-call-ip6tables"
sudo sh -c "echo 'net.bridge.bridge-nf-call-iptables=1' >> /etc/sysctl.conf"
sudo sh -c "echo 'net.bridge.bridge-nf-call-ip6tables=1' >> /etc/sysctl.conf"

# Install dependencies for docker-ce
sudo yum -y install yum-utils device-mapper-persistent-data lvm2

# Add the docker-ce repository
sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo

# Add the Kubernetes Repository
sudo sh -c 'cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://packages.cloud.google.com/yum/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
       https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOF'

# Update yum cache after adding repository
sudo yum -y makecache fast

# Install latest supported docker runtime (18.06 is the latest runtime supported by Kubernetes v1.13.2)
sudo yum -y install docker-ce-18.06.1.ce

# Install Kubernetes
sudo yum -y install kubelet kubeadm kubectl

# Enable kubectl bash-completion
sudo yum -y install bash-completion
source <(kubectl completion bash)
echo "source <(kubectl completion bash)" >> ~/.bashrc

# Enable docker and kubelet services
sudo systemctl enable docker.service
sudo systemctl enable kubelet.service


# Check what cgroup driver that docker is using
sudo docker info | grep -i cgroup

# Add the cgroup driver from the previous step to the kublet config as an extra argument
sudo sed -i "s/^\(KUBELET_EXTRA_ARGS=\)\(.*\)$/\1\"--cgroup-driver=$(sudo docker info | grep -i cgroup | cut -d" " -f3)\2\"/" /etc/sysconfig/kubelet



########################
# === Workers Only === #
########################
# Join worker node to k8s cluster using the token and discovery-token-ca-cert-hash from master
sudo kubeadm join ${MASTER_HOSTNAME}:6443 --token ${TOKEN} --discovery-token-ca-cert-hash sha256:${DISCOVERY_TOKEN_CA_CERT_HASH}


							`)
			fmt.Println("creating instance", i)
			if j == "lead" {
				NodeUD = LUD
			}
			EmptyString, err := ec2.NewInstance(ctx, j, &ec2.InstanceArgs{
				Ami:                      pulumi.String("ami-01e24c1756b2c7bd5"),
				InstanceType:             pulumi.String("t3.micro"),
				AssociatePublicIpAddress: pulumi.Bool(true),
				// Manually create aws kypair in the name "aws-key"
				KeyName:  pulumi.String("aws-key"),
				SubnetId: mySubnet.ID(),
				UserData: NodeUD,
				Tags: pulumi.StringMap{
					"Name": pulumi.String(j),
				},
			})
			if err != nil {
				return err
			}
			print(EmptyString)
		}
		return nil
	})
}
