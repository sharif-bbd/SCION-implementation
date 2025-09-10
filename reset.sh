mkdir -p logs
cd scion
sudo service docker start
./scion.sh bazel-remote
cd ..
python3 topo-generator.py