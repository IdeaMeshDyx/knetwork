#!/usr/bin/env bash

#   Copyright The containerd Authors.

#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at

#       http://www.apache.org/licenses/LICENSE-2.0

#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.

#
# Builds and installs cni plugins to /opt/cni/bin,
# and create basic cni config in /etc/cni/net.d.
#

set -e -x -u
sudo apt-get update && sudo apt-get install -y bridge-utils


# Install Golang
wget --quiet https://storage.googleapis.com/golang/go1.9.1.linux-amd64.tar.gz
sudo tar -zxf go1.9.1.linux-amd64.tar.gz -C /usr/local/
echo 'export GOROOT=/usr/local/go' >> /home/vagrant/.bashrc
echo 'export GOPATH=$HOME/go' >> /home/vagrant/.bashrc
echo 'export PATH=$PATH:$GOROOT/bin:$GOPATH/bin' >> /home/vagrant/.bashrc
export GOROOT=/usr/local/go
export GOPATH=$HOME/go
export PATH=$PATH:$GOROOT/bin:$GOPATH/bin
mkdir -p /home/vagrant/go/src
rm -rf /home/vagrant/go1.9.1.linux-amd64.tar.gz


# Download CNI and CNI plugins binaries
wget --quiet https://github.com/containernetworking/cni/releases/download/v0.6.0/cni-amd64-v0.6.0.tgz
wget --quiet https://github.com/containernetworking/plugins/releases/download/v0.6.0/cni-plugins-amd64-v0.6.0.tgz
sudo mkdir -p /opt/cni/bin
sudo mkdir -p /etc/cni/net.d
sudo tar -zxf cni-amd64-v0.6.0.tgz -C /opt/cni/bin
sudo tar -zxf cni-plugins-amd64-v0.6.0.tgz -C /opt/cni/bin
rm -rf /home/vagrant/cni-plugins-amd64-v0.6.0.tgz /home/vagrant/cni-amd64-v0.6.0.tgz

# Clone this example repository
git clone https://github.com/hwchiu/CNI_Tutorial_2018 go/src/github.com/hwchiu/CNI_Tutorial_2018
go get -u github.com/kardianos/govendor
cd go/src/github.com/hwchiu/CNI_Tutorial_2018
govendor sync
