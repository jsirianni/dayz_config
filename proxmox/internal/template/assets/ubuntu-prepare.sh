#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
cloud-init status --wait
apt-get update
apt-get install -y cloud-init qemu-guest-agent openssh-server
systemctl enable qemu-guest-agent ssh
mkdir -p /etc/cloud/cloud.cfg.d /var/lib/pvecli
cat > /etc/cloud/cloud.cfg.d/99-pvecli.cfg <<'EOF'
datasource_list: [ NoCloud ]
ssh_deletekeys: true
system_info:
  default_user:
    name: pveadmin
    lock_passwd: true
    gecos: Proxmox administrator
    groups: [adm, sudo]
    sudo: ["ALL=(ALL) NOPASSWD:ALL"]
    shell: /bin/bash
EOF
rm -f /etc/cloud/cloud.cfg.d/subiquity-disable-cloudinit-networking.cfg /etc/cloud/cloud.cfg.d/99-disable-network-config.cfg
rm -f /etc/netplan/*.yaml
cloud-init clean --logs --machine-id --seed
rm -f /etc/ssh/ssh_host_* /var/lib/dbus/machine-id
ln -s /etc/machine-id /var/lib/dbus/machine-id
rm -f /var/lib/dhcp/* /var/lib/NetworkManager/*lease* /etc/sudoers.d/99-pvecli-build
rm -f /root/.ssh/authorized_keys /root/.bash_history
userdel --force --remove pvebuild
rm -rf /var/log/installer
printf '%s' '{{.Operation}}' > /var/lib/pvecli/sealed
rm -f /tmp/pvecli-prepare.sh
printf 'pvecli-prepared\n'
