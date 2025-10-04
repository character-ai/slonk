#!/bin/bash

set -e

EROOT=/home/ephemeral
LOCKFILE=$EROOT/.cleanup.lock

# Make sure ephemeral directories exist and are well chmod'd

GROUP_NAME=${SLURM_GROUP:-"users"}
for EPATH in $EROOT $EROOT/1d $EROOT/7d $EROOT/30d
do
    mkdir -p $EPATH
    chown root:${GROUP_NAME} $EPATH
    chmod a+rwx $EPATH
    chmod g+s $EPATH
done

# lock
if [ -e $LOCKFILE ]; then
    if [ $(find "$LOCKFILE" -mmin +360) ]; then
        # lockfile has been held for 6 hours. assume we crashed
        echo "The lockfile has been held for 6 hours. Assuming we crashed."
        rm -f $LOCKFILE
    else
        echo "The lockfile is held. Cowardly exiting."
        exit 1
    fi
fi
touch $LOCKFILE

# cleanup files
find  $EROOT/1d  -type f -mtime +1  -print0| xargs -0 --max-procs 8 rm -f
find  $EROOT/7d  -type f -mtime +7  -print0| xargs -0 --max-procs 8 rm -f
find  $EROOT/30d -type f -mtime +30 -print0| xargs -0 --max-procs 8 rm -f

# cleanup all empty directories
find  $EROOT/1d  -type d -mtime +1  -print0| xargs -0 --max-procs 8 rmdir
find  $EROOT/7d  -type d -mtime +7  -print0| xargs -0 --max-procs 8 rmdir
find  $EROOT/30d -type d -mtime +30 -print0| xargs -0 --max-procs 8 rmdir

# cleanup lockfile
rm $LOCKFILE

