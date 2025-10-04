#!/bin/sh

if [ ! -x /usr/sbin/nfsstat ]
then
    exit 0
fi

TOTAL_OPS=$(nfsstat -c -3 -l | grep total | sed "s/^.*: //")
curl --silent --header 'Content-Type: application/json' --request POST --data "[{\"__name__\": \"nfs_ops\", \"__value__\": $TOTAL_OPS}]" http://localhost:7080/write > /dev/null
