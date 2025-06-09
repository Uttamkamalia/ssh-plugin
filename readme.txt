127.10.0.0/22 → gives 1024 usable IPs
127.10.0.1 → 127.10.3.254




go run generate_input.go

sudo bash ./add_ips_to_koopback.sh

go run start_mockservers.go  or go run main.go --exclude 127.10.0.5,127.10.1.100


go  build ssh_plugin.go ssh_utils.go

sudo setcap cap_net_raw+ep ./ssh_plugin


# DISCOVERY
./ssh_plugin DISCOVERY test-samples/discovery-input-sample.json test-samples/discovery-result.json

# POLLING
./ssh_plugin POLLING {\"discovery_profile_id\": 1,\"discovery_batch_job_id\": \"0a23ae49-bf55-4d0a-aae6-4fa6ccb52595\",\"metric_ids\":[\"cpu_usage\", \"memory_usage\"],\"devices\": [{\"protocol\": \"SSH\",\"ip\": \"127.10.0.5\",\"port\": 2222,\"credential\": {\"username\": \"username\",\"password\": \"password\"}}]}
