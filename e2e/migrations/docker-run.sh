CURRENT_DIRECTORY=`pwd`

docker run --privileged -v $CURRENT_DIRECTORY:/gopkg/src/migrations -it nomad-e2e /bin/bash -c "cd gopkg/src/migrations && go test"
