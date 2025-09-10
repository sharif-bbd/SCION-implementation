brew install lima
# brew install wget
mkdir /Users/$USER/scion_lima
mkdir -p ~/.lima/scion_lima
cp vm/lima/lima.yaml ~/.lima/scion_lima/lima.yaml
cp -r . /Users/$USER/scion_lima/scion_lima
cd /Users/$USER/scion_lima/scion_lima
# wget https://go.dev/dl/go1.24.2.linux-arm64.tar.gz -O ../go1.24.2.linux-arm64.tar.gz
limactl start scion_lima
limactl shell scion_lima /home/scion_lima/scion_lima/vm/lima/lima_vm_setup_internal.sh