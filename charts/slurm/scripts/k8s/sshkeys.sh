#!/bin/bash
set -e
u=$1
if [ -z "$u" ]
then
    exit
fi
shift
grep "^$u:" "$(dirname $0)/sshkeys.txt" | sed "s/^$u://"
# set -e will prevent providing the internal key if there were no known public keys
cat /etc/id-rsa-cluster/id_rsa_cluster.pub
echo
