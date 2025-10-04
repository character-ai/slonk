#!/bin/bash

# Auto restarts the slurm controller if slurm.conf is updated

# Path to the file you want to monitor
FILE="/etc/slurm/slurm.conf"

# File where the last known hash is stored
HASH_FILE="/var/run/slurm.hash"

# Compute the current hash of the file
CURRENT_HASH=$(sha256sum "$FILE" | awk '{print $1}')

# Check if the HASH_FILE exists. If not, create it and store the current hash.
if [ ! -f "$HASH_FILE" ]; then
    echo "$CURRENT_HASH" > "$HASH_FILE"
    exit 0
fi

# Read the last known hash from the HASH_FILE
LAST_HASH=$(cat "$HASH_FILE")

# Compare the current hash with the last known hash
if [ "$CURRENT_HASH" != "$LAST_HASH" ]; then
    # Run the command if the file has changed
    scontrol reconfigure

    # Update the last known hash
    echo "$CURRENT_HASH" > "$HASH_FILE"
fi

