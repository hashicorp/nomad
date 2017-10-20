## End to end tests for migrating data in sticky volumes

These tests run in a docker container to ensure proper setup/teardown.

To create the testing image:
`./docker-init.sh`

To run tests:
`./docker-run.sh`

TODO:
  1. Specify how many servers/clients in the test
  2. Have a callback to specify the client options
  3. Run servers/clients in the docker container, return IP addresses for each
     instance, but have the test run on the host.

