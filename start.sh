pkill -f verifier-app || true
pkill -f client-app || true
sleep 2
trap "echo 'Stopping all processes...'; kill 0; exit 1" SIGINT

./verifier-app --config=config/verifier-1.toml > ./logs/verifier-1.log 2>&1 &
sleep 1
timeout 20s ./client-app --local=127.0.0.43 --remote=1-ff00:0:113,[fd00:f00d:cafe::7f00:23]:31255> ./logs/client-1.log 2>&1

./verifier-app --config=config/verifier-2.toml > ./logs/verifier-2.log 2>&1 &
sleep 1
timeout 20s ./client-app --local=fd00:f00d:cafe::7f00:23 --remote=2-ff00:0:213,127.0.0.52:31255> ./logs/client-2.log 2>&1