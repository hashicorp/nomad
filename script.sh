#!/usr/bin/env bash

set -o errexit

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

# Create test namespaces.
nomad namespace apply -description "test namespace" test-1
nomad namespace apply -description "test namespace" test-2
nomad namespace apply -description "test namespace" test-3
nomad namespace apply -description "test namespace" test-4
nomad namespace apply -description "test namespace" test-5
nomad namespace apply -description "test namespace" test-6
nomad namespace apply -description "test namespace" test-7
nomad namespace apply -description "test namespace" test-8
nomad namespace apply -description "test namespace" test-9

# Create test ACL policies.
nomad acl policy apply -description "test acl policy" test-1 "$SCRIPT_DIR"/_test_acl_policy.hcl
nomad acl policy apply -description "test acl policy" test-2 "$SCRIPT_DIR"/_test_acl_policy.hcl
nomad acl policy apply -description "test acl policy" test-3 "$SCRIPT_DIR"/_test_acl_policy.hcl
nomad acl policy apply -description "test acl policy" test-4 "$SCRIPT_DIR"/_test_acl_policy.hcl
nomad acl policy apply -description "test acl policy" test-5 "$SCRIPT_DIR"/_test_acl_policy.hcl
nomad acl policy apply -description "test acl policy" test-6 "$SCRIPT_DIR"/_test_acl_policy.hcl
nomad acl policy apply -description "test acl policy" test-7 "$SCRIPT_DIR"/_test_acl_policy.hcl
nomad acl policy apply -description "test acl policy" test-8 "$SCRIPT_DIR"/_test_acl_policy.hcl
nomad acl policy apply -description "test acl policy" test-9 "$SCRIPT_DIR"/_test_acl_policy.hcl

# Create client ACL tokens.
nomad acl token create -name="test client acl token" -policy=test-1 -type=client
nomad acl token create -name="test client acl token" -policy=test-2 -type=client
nomad acl token create -name="test client acl token" -policy=test-3 -type=client
nomad acl token create -name="test client acl token" -policy=test-4 -type=client
nomad acl token create -name="test client acl token" -policy=test-5 -type=client
nomad acl token create -name="test client acl token" -policy=test-6 -type=client
nomad acl token create -name="test client acl token" -policy=test-7 -type=client
nomad acl token create -name="test client acl token" -policy=test-8 -type=client
nomad acl token create -name="test client acl token" -policy=test-9 -type=client

# Create management ACL tokens.
nomad acl token create -name="test management acl token" -type=management
nomad acl token create -name="test management acl token" -type=management
nomad acl token create -name="test management acl token" -type=management
nomad acl token create -name="test management acl token" -type=management
nomad acl token create -name="test management acl token" -type=management
nomad acl token create -name="test management acl token" -type=management
nomad acl token create -name="test management acl token" -type=management
nomad acl token create -name="test management acl token" -type=management
nomad acl token create -name="test management acl token" -type=management
