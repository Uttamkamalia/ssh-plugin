#!/bin/bash

SUBNET_PREFIX="127.10"

for i in {0..3}; do
  for j in {1..254}; do
    IP="${SUBNET_PREFIX}.${i}.${j}"

    # Delete any iptables rule matching the IP (in INPUT chain, adjust as needed)
    sudo iptables -D INPUT -s $IP -j ACCEPT 2>/dev/null
    sudo iptables -D INPUT -d $IP -j ACCEPT 2>/dev/null

    # If you have other chains, repeat similarly or use iptables-save/restore approach
  done
done

echo "Removed iptables entries for 127.10.0.0/22 IPs"
