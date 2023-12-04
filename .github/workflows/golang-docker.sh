#!/bin/bash

# Checkout a different branch
git fetch origin
git checkout fix-gosdk-version

# Start mongo before starting keploy.
sudo docker run --name mongoDb --rm  -p 27017:27017 -d mongo

# Generate the keploy-config file.
./../../keployv2 generate-config

# Update the global noise to ts.
config_file="./keploy-config.yaml"
sed -i 's/"body": {}/"body": {"ts":[]}/' "$config_file"

# Remove any preexisting keploy tests and mocks.
sudo rm -rf keploy/

# Start keploy in record mode.
docker network create keploy-network
docker run  --name keploy-v2 -p 16789:16789 --privileged --pid=host -v "$(pwd)":/files -v /sys/fs/cgroup:/sys/fs/cgroup -v /sys/kernel/debug:/sys/kernel/debug -v /sys/fs/bpf:/sys/fs/bpf -v /var/run/docker.sock:/var/run/docker.sock --rm keployv2 record -c 'go run main.go handler.go' &

# Wait for the application to start.
app_started=false
while [ "$app_started" = false ]; do
    if curl -X GET http://localhost:8080/CJBKJd92; then
        app_started=true
    fi
    sleep 3 # wait for 3 seconds before checking again.
done

# Get the pid of the application.
pid=$(pgrep keploy)

# Start making curl calls to record the testcases and mocks.
curl --request POST \
  --url http://localhost:8080/url \
  --header 'content-type: application/json' \
  --data '{
  "url": "https://google.com"
}'

curl --request POST \
  --url http://localhost:8080/url \
  --header 'content-type: application/json' \
  --data '{
  "url": "https://facebook.com"
}'

curl -X GET http://localhost:8080/CJBKJd92

# Wait for 5 seconds for keploy to record the tcs and mocks.
sleep 5

# Stop keploy.
sudo kill $pid

# Start the keploy in test mode.
docker run  --name keploy-v2 -p 16789:16789 --privileged --pid=host -v "$(pwd)":/files -v /sys/fs/cgroup:/sys/fs/cgroup -v /sys/kernel/debug:/sys/kernel/debug -v /sys/fs/bpf:/sys/fs/bpf -v /var/run/docker.sock:/var/run/docker.sock --rm keployv2 record -c 'go run main.go handler.go'

# Get the test results from the testReport file.
report_file="./keploy/testReports/report-1.yaml"
test_status=$(grep 'status:' "$report_file" | head -n 1 | awk '{print $2}')

# Return the exit code according to the status.
if [ "$test_status" = "PASSED" ]; then
    exit 0
else
    exit 1
fi