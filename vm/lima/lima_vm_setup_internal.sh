sudo apt -y update
sudo apt -y install build-essential libssl-dev zlib1g-dev libbz2-dev libreadline-dev libsqlite3-dev curl wget libncursesw5-dev xz-utils tk-dev libxml2-dev libxmlsec1-dev libffi-dev liblzma-dev
curl -fsSL https://pyenv.run | bash
echo 'export PYENV_ROOT="$HOME/.pyenv"' >> ~/.bashrc
echo '[[ -d $PYENV_ROOT/bin ]] && export PATH="$PYENV_ROOT/bin:$PATH"' >> ~/.bashrc
echo 'eval "$(pyenv init - bash)"' >> ~/.bashrc
echo 'export PYENV_ROOT="$HOME/.pyenv"' >> ~/.profile
echo '[[ -d $PYENV_ROOT/bin ]] && export PATH="$PYENV_ROOT/bin:$PATH"' >> ~/.profile
echo 'eval "$(pyenv init - bash)"' >> ~/.profile
export PYENV_ROOT="$HOME/.pyenv"
[[ -d $PYENV_ROOT/bin ]] && export PATH="$PYENV_ROOT/bin:$PATH"
eval "$(pyenv init - bash)"
pyenv install 3.10
pyenv global 3.10
which python3

sudo apt-get update
sudo apt-get install ca-certificates curl
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "${UBUNTU_CODENAME:-$VERSION_CODENAME}") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update
sudo apt-get -y install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo chmod 666 /var/run/docker.sock
wget https://go.dev/dl/go1.24.2.linux-arm64.tar.gz -O /home/scion_lima/go1.24.2.linux-arm64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf /home/scion_lima/go1.24.2.linux-arm64.tar.gz
export PATH=$PATH:/usr/local/go/bin
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
go version
cd /home/scion_lima/scion_lima/scion
./tools/install_bazel
export PATH=~/.local/bin:$PATH
echo 'export PATH=~/.local/bin:$PATH' >> ~/.profile
echo 'export PATH=~/.local/bin:$PATH' >> ~/.bashrc
./tools/install_deps
./scion.sh bazel-remote
make
cd ..
pip3 --disable-pip-version-check install --user --require-hashes -r "scion/tools/env/pip3/requirements.txt" --break-system-packages
# sed -i 's/amd64/arm64/g' build.sh
# chmod +x build.sh

# ./reset.sh
# ./startTopology.sh
# ./build.sh
# ./start.sh
# ./stop.sh
# ./stopTopology.sh


