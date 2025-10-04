#!/bin/bash
LOCKFILE=/tmp/.localdisk-cleanup.lock
CLEANUP_FOLDER=/mnt/localdisk/cache

if [ -e $LOCKFILE ]; then
    echo "The lockfile is held. Cowardly exiting."
    exit 1
fi
touch $LOCKFILE

if [ ! -e $CLEANUP_FOLDER ]; then
    rm $LOCKFILE
    exit 0
fi

find /mnt/localdisk/cache -type f -mmin +360 -print0 | xargs -0 --max-procs 8 rm

rm $LOCKFILE
