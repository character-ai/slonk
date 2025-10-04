#!/usr/bin/env python3

DRAINED=$(sinfo | grep drain | awk '{print $6}')
echo "Restarting drained nodes $DRAINED..."
scontrol show hostnames "$DRAINED" | xargs -I{} -P 128 kubectl delete pod {}
