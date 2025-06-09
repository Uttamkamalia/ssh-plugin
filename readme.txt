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
./ssh_plugin POLLING "{\"discovery_profile_id\":1,\"metric_ids\":[\"memory_usage\", \"cpu_usage\"],\"config\":{\"concurrency\":10,\"device_timeout\":21,\"ping_timeout\":1,\"port_timeout\":1,\"ssh_timeout\":5,\"ping_retries\":1,\"port_retries\":1,\"ssh_retries\":1,\"retry_backoff\":1},\"devices\":[{\"device_type_id\":1,\"device_id\":2,\"metric_group_id\":1,\"protocol\":\"SSH\",\"ip\":\"127.10.1.234\",\"port\":2222,\"credential\":{\"username\":\"username\",\"password\":\"password\"}},{\"device_type_id\":1,\"device_id\":2,\"metric_group_id\":1,\"protocol\":\"SSH\",\"ip\":\"127.10.1.235\",\"port\":2222,\"credential\":{\"username\":\"username\",\"password\":\"password\"}},{\"device_type_id\":1,\"device_id\":2,\"metric_group_id\":1,\"protocol\":\"SSH\",\"ip\":\"127.10.1.236\",\"port\":2222,\"credential\":{\"username\":\"username\",\"password\":\"password\"}},{\"device_type_id\":1,\"device_id\":2,\"metric_group_id\":1,\"protocol\":\"SSH\",\"ip\":\"127.10.1.237\",\"port\":2222,\"credential\":{\"username\":\"username\",\"password\":\"password\"}},{\"device_type_id\":1,\"device_id\":2,\"metric_group_id\":1,\"protocol\":\"SSH\",\"ip\":\"127.10.1.238\",\"port\":2222,\"credential\":{\"username\":\"username\",\"password\":\"password\"}},{\"device_type_id\":1,\"device_id\":2,\"metric_group_id\":1,\"protocol\":\"SSH\",\"ip\":\"127.10.1.239\",\"port\":2222,\"credential\":{\"username\":\"username\",\"password\":\"password\"}},{\"device_type_id\":1,\"device_id\":2,\"metric_group_id\":1,\"protocol\":\"SSH\",\"ip\":\"127.10.1.240\",\"port\":2222,\"credential\":{\"username\":\"username\",\"password\":\"password\"}},{\"device_type_id\":1,\"device_id\":2,\"metric_group_id\":1,\"protocol\":\"SSH\",\"ip\":\"127.10.1.241\",\"port\":2222,\"credential\":{\"username\":\"username\",\"password\":\"password\"}},{\"device_type_id\":1,\"device_id\":2,\"metric_group_id\":1,\"protocol\":\"SSH\",\"ip\":\"127.10.1.242\",\"port\":2222,\"credential\":{\"username\":\"username\",\"password\":\"password\"}},{\"device_type_id\":1,\"device_id\":2,\"metric_group_id\":1,\"protocol\":\"SSH\",\"ip\":\"127.10.1.243\",\"port\":2222,\"credential\":{\"username\":\"username\",\"password\":\"password\"}}]}"
