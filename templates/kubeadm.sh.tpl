set -e

sudo apt-get update && sudo apt-get install -y apt-transport-https curl
sudo curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key add -

sudo bash -c "cat << EOF > /etc/apt/sources.list.d/kubernetes.list
deb https://apt.kubernetes.io/ kubernetes-xenial main
EOF"

sudo apt-get update
sudo apt-get install -y kubelet={{ .K8SVersion}}-00 kubeadm={{ .K8SVersion}}-00 kubectl={{ .K8SVersion}}-00 --allow-unauthenticated
sudo apt-mark hold kubelet kubeadm kubectl

sudo systemctl daemon-reload
sudo systemctl restart kubelet

HOSTNAME="$(hostname)"
{{ if eq .Provider "aws" }}
HOSTNAME="$(hostname -f)"
{{ end }}

# TODO: place ca/kubeblet certificates to the custom dir to make this step idempotent.
#       `kubeadm reset` removes this certificates and then creates a new one.


sudo mkdir -p /etc/supergiant

{{if .IsMaster }}
{{ if .IsBootstrap }}

sudo bash -c "cat << EOF > /etc/supergiant/kubeadm.conf
---
apiVersion: kubeadm.k8s.io/v1beta1
kind: InitConfiguration
localAPIEndpoint:
  bindPort: 443
nodeRegistration:
  kubeletExtraArgs:
    node-ip: {{ .PrivateIP }}
{{ if .Provider }}    cloud-provider: {{ .Provider }}{{ end }}
---
apiVersion: kubeadm.k8s.io/v1beta1
kind: ClusterConfiguration
kubernetesVersion: v{{ .K8SVersion }}
clusterName: kubernetes
controlPlaneEndpoint: {{ .InternalDNSName }}
certificatesDir: /etc/kubernetes/pki
apiServer:
  certSANs:
  - {{ .ExternalDNSName }}
  - {{ .InternalDNSName }}
  extraArgs:
    authorization-mode: Node,RBAC
{{ if .Provider }}    cloud-provider: {{ .Provider }}{{ end }}
  timeoutForControlPlane: 8m0s
controllerManager:
  extraArgs:
{{ if .Provider }}    cloud-provider: {{ .Provider }}{{ end }}
scheduler:
  extraArgs:
dns:
  type: CoreDNS
etcd:
  local:
    dataDir: /var/lib/etcd
networking:
  dnsDomain: cluster.local
  podSubnet: {{ .CIDR }}
  serviceSubnet: {{ .ServiceCIDR }}
EOF"

sudo kubeadm init --ignore-preflight-errors=NumCPU \
--node-name ${HOSTNAME} \
--config=/etc/supergiant/kubeadm.conf
{{ else }}

sudo bash -c "cat << EOF > /etc/supergiant/kubeadm.conf
apiVersion: kubeadm.k8s.io/v1beta1
kind: JoinConfiguration
nodeRegistration:
  kubeletExtraArgs:
    node-ip: {{ .PrivateIP }}
    {{ if .Provider }}cloud-provider: {{ .Provider }}{{ end }}
discovery:
  bootstrapToken:
    token: {{ .Token }}
    apiServerEndpoint: {{ .InternalDNSName }}:443
    unsafeSkipCAVerification: true
controlPlane:
  localAPIEndpoint:
    bindPort: 443
EOF"

sudo kubeadm config images pull
sudo kubeadm join --ignore-preflight-errors=NumCPU {{ .InternalDNSName }}:443 \
--node-name ${HOSTNAME} \
--config=/etc/supergiant/kubeadm.conf
{{ end }}

sudo mkdir -p $HOME/.kube
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config
{{ else }}

sudo bash -c "cat << EOF > /etc/supergiant/kubeadm.conf
apiVersion: kubeadm.k8s.io/v1beta1
kind: JoinConfiguration
nodeRegistration:
  kubeletExtraArgs:
    node-ip: {{ .PrivateIP }}
{{ if .Provider }}    cloud-provider: {{ .Provider }}{{ end }}
discovery:
  bootstrapToken:
    token: {{ .Token }}
    apiServerEndpoint: {{ .InternalDNSName }}:443
    unsafeSkipCAVerification: true
EOF"

sudo kubeadm join --ignore-preflight-errors=NumCPU {{ .InternalDNSName }}:443 \
--node-name ${HOSTNAME} \
--config=/etc/supergiant/kubeadm.conf
{{ end }}
