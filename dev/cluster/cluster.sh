# launch server 
( nomad agent -config=server1.hcl 2>&1 | tee "/tmp/server1/log" ; echo "Exit code: $?" >> "/tmp/server1/log" ) &

# launch client 1
( nomad agent -config=client1.hcl 2>&1 | tee "/tmp/client1/log" ; echo "Exit code: $?" >> "/tmp/client1/log" ) &

# launch client 2
( nomad agent -config=client2.hcl 2>&1 | tee "/tmp/client2/log" ; echo "Exit code: $?" >> "/tmp/client2/log" ) &

# launch client 3
( nomad agent -config=client3.hcl 2>&1 | tee "/tmp/client3/log" ; echo "Exit code: $?" >> "/tmp/client3/log" ) &

# launch client 4
( nomad agent -config=client4.hcl 2>&1 | tee "/tmp/client4/log" ; echo "Exit code: $?" >> "/tmp/client4/log" ) &

