#! /bin/bash

# ./../.github/workflows/fake-iid.sh

# Start postgres instance.
docker network create keploy-network
docker run -d -e POSTGRES_USER=petclinic -e POSTGRES_PASSWORD=petclinic -e POSTGRES_DB=petclinic -p 5432:5432 --name mypostgres --network keploy-network postgres:15.2

# Checkout the add-petclinic branch.
git fetch origin
git checkout add-petclinic

# Update the java version
source ./../.github/workflows/update-java.sh
mvn --version
cd ./spring-petclinic/spring-petclinic-rest

# Remove any existing test and mocks by keploy.
sudo rm -rf keploy/

# Start keploy in record mode.
sudo mvn --version
sudo java --version
mvn clean install -Dmaven.test.skip=true
ls target/
sudo docker build -t java-app:1.0 .
pwd
echo "checking the connections again"
cat src/main/resources/application-postgresql.properties
docker run  --name keploy-v2 -p 16789:16789 --privileged --pid=host -v "$(pwd)":/files -v /sys/fs/cgroup:/sys/fs/cgroup -v /sys/kernel/debug:/sys/kernel/debug -v /sys/fs/bpf:/sys/fs/bpf -v /var/run/docker.sock:/var/run/docker.sock --rm keployv2 record -c 'docker run -p 9966:9966 --name javaApp --network keploy-network java-app:1.0' --debug  &
sleep 3
docker cp ./src/main/resources/db/postgresql/initDB.sql mypostgres:/initDB.sql
docker exec mypostgres psql -U petclinic -d petclinic -f /initDB.sql

# Wait for the application to start.
app_started=false
while [ "$app_started" = false ]; do
    if curl -X GET http://localhost:9966/petclinic/api/pettypes; then
        app_started=true
    fi
    sleep 3 # wait for 3 seconds before checking again.
done

# Get the pid of the application.
pid=$(pgrep keploy)

# Start making curl calls to record the testcases and mocks.
curl -X GET http://localhost:9966/petclinic/api/pettypes

curl --request POST \
--url http://localhost:9966/petclinic/api/pettypes \
   --header 'content-type: application/json' \
   --data '{
    "name":"John Doe"}'

curl -X GET http://localhost:9966/petclinic/api/pettypes

curl --request POST \
--url http://localhost:9966/petclinic/api/pettypes \
   --header 'content-type: application/json' \
   --data '{
    "name":"Alice Green"}'

curl -X GET http://localhost:9966/petclinic/api/pettypes

 curl --request DELETE \
--url http://localhost:9966/petclinic/api/pettypes/1

curl -X GET http://localhost:9966/petclinic/api/pettypes

# Wait for 5 seconds for keploy to record the tcs and mocks.
sleep 5

# Stop keploy.
sudo docker rm -f keploy-v2
sudo docker rm -f javaApp

# checking the mocks
cat ./keploy/test-set-0/mocks.yaml
cat ./keploy/test-set-1/mocks.yaml

# Start keploy in test mode.
sudo docker run  --name keploy-v2 -p 16789:16789 --privileged --pid=host -v "$(pwd)":/files -v /sys/fs/cgroup:/sys/fs/cgroup -v /sys/kernel/debug:/sys/kernel/debug -v /sys/fs/bpf:/sys/fs/bpf -v /var/run/docker.sock:/var/run/docker.sock --rm keployv2 test -c 'docker run -p 9966:9966 --name javaApp --network keploy-network java-app:1.0' --debug --delay 100 --apiTimeout 10

# Get the test results from the testReport file.
report_file="./keploy/testReports/report-1.yaml"
test_status=$(grep 'status:' "$report_file" | head -n 1 | awk '{print $2}')

# Return the exit code according to the status.
if [ "$test_status" = "PASSED" ]; then
    exit 0
else
    exit 1
fi