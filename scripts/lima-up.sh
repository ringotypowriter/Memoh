#!/usr/bin/env sh
set -e

if [ "$(uname -s)" != "Darwin" ]; then
  exit 0
fi

limactl start default

if ! limactl shell default -- sh -lc 'command -v memoh-cli >/dev/null 2>&1'; then
  vm_arch=$(limactl shell default -- uname -m)
  if [ "$vm_arch" = "aarch64" ] || [ "$vm_arch" = "arm64" ]; then
    go_arch="arm64"
  else
    go_arch="amd64"
  fi
  bin_path="/tmp/memoh-cli-linux-$go_arch"
  GOOS=linux GOARCH=$go_arch go build -trimpath -ldflags "-s -w" -o "$bin_path" ./cmd/cli
  limactl shell default -- sudo -n mkdir -p /usr/local/bin
  limactl shell default -- sudo -n tee /usr/local/bin/memoh-cli >/dev/null < "$bin_path"
  limactl shell default -- sudo -n chmod +x /usr/local/bin/memoh-cli
fi

limactl shell default -- sh -lc 'command -v curl >/dev/null 2>&1' || {
  echo "curl not found in Lima VM; install curl and rerun"
  exit 1
}

limactl shell default -- sh -lc 'test -x /opt/cni/bin/bridge' || {
  vm_arch=$(limactl shell default -- uname -m)
  if [ "$vm_arch" = "aarch64" ] || [ "$vm_arch" = "arm64" ]; then
    cni_arch="arm64"
  else
    cni_arch="amd64"
  fi
  url="https://github.com/containernetworking/plugins/releases/download/v1.9.0/cni-plugins-linux-${cni_arch}-v1.9.0.tgz"
  limactl shell default -- sudo -n mkdir -p /opt/cni/bin
  limactl shell default -- sudo -n curl -L -o /tmp/cni-plugins.tgz "$url"
  limactl shell default -- sudo -n tar -C /opt/cni/bin -xzf /tmp/cni-plugins.tgz
}

limactl shell default -- sudo -n mkdir -p /etc/cni/net.d
limactl shell default -- sudo -n sh -lc 'test -f /etc/cni/net.d/10-memoh-bridge.conflist' || \
limactl shell default -- sudo -n sh -lc 'printf "%s\n" "{" "  \"cniVersion\": \"0.4.0\"," "  \"name\": \"memoh-bridge\"," "  \"plugins\": [" "    {" "      \"type\": \"bridge\"," "      \"bridge\": \"cni0\"," "      \"isGateway\": true," "      \"ipMasq\": true," "      \"promiscMode\": false," "      \"hairpinMode\": true," "      \"ipam\": {" "        \"type\": \"host-local\"," "        \"subnet\": \"10.88.0.0/16\"," "        \"routes\": [" "          {\"dst\": \"0.0.0.0/0\"}" "        ]" "      }" "    }," "    {\"type\": \"portmap\", \"capabilities\": {\"portMappings\": true}}," "    {\"type\": \"firewall\"}," "    {\"type\": \"tuning\"}" "  ]" "}" > /etc/cni/net.d/10-memoh-bridge.conflist'
