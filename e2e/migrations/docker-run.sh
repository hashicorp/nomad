CURRENT_DIRECTORY=`pwd`
ROOT_DIRECTORY="$( dirname "$(dirname "$CURRENT_DIRECTORY")")"

testOne= docker run --privileged -v \
$CURRENT_DIRECTORY:/gopkg/src/github.com/hashicorp/nomad \
-it nomad-e2e /bin/bash \
-c "cd gopkg/src/github.com/hashicorp/nomad/e2e/migrations && go test --run \
TestJobMigrations -integration"

echo $testOne

docker run --privileged \
-v $CURRENT_DIRECTORY:/gopkg/src/github.com/hashicorp/nomad \
-it nomad-e2e /bin/bash \
-c "cd gopkg/src/github.com/hashicorp/nomad/e2e/migrations && go test --run \
TestMigrations_WithACLs"
