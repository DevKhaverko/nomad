#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -e

rm -rf /tmp/nomad-dev-cluster/
mkdir -p /tmp/nomad-dev-cluster/server{1,2,3} /tmp/nomad-dev-cluster/client{1,2}


DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# launch server
( ../../bin/nomad agent -config=${DIR}/server1.hcl 2>&1 | tee "/tmp/nomad-dev-cluster/server1/log" ; echo "Exit code: $?" >> "/tmp/nomad-dev-cluster/server1/log" ) &

( ../../bin/nomad agent -config=${DIR}/server2.hcl 2>&1 | tee "/tmp/nomad-dev-cluster/server2/log" ; echo "Exit code: $?" >> "/tmp/nomad-dev-cluster/server2/log" ) &

( ../../bin/nomad agent -config=${DIR}/server3.hcl 2>&1 | tee "/tmp/nomad-dev-cluster/server3/log" ; echo "Exit code: $?" >> "/tmp/nomad-dev-cluster/server3/log" ) &

# launch client 1
( ../../bin/nomad agent -config=${DIR}/client1.hcl 2>&1 | tee "/tmp/nomad-dev-cluster/client1/log" ; echo "Exit code: $?" >> "/tmp/nomad-dev-cluster/client1/log" ) &

# launch client 2
( ../../bin/nomad agent -config=${DIR}/client2.hcl 2>&1 | tee "/tmp/nomad-dev-cluster/client2/log" ; echo "Exit code: $?" >> "/tmp/nomad-dev-cluster/client2/log" ) &


trap 'kill -SIGTERM $(jobs -pr)' SIGINT SIGTERM

wait

# wait again to ensure process die
wait
