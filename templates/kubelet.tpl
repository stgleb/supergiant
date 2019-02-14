sudo bash -c "cat > /etc/default/kubelet <<EOF
KUBELET_EXTRA_ARGS=--tls-cert-file=/etc/kubernetes/pki/kubelet.crt --tls-private-key-file=/etc/kubernetes/pki/kubelet.key
EOF"

sudo systemctl daemon-reload
sudo systemctl restart kubelet