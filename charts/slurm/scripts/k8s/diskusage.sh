#/bin/bash
set -e
ncdu -o - /home --exclude .git --exclude .conda --exclude .cache | zstd -c - | pv - > /home/common/.filereport.ncdu.zstd.inprogress
mv -f /home/common/.filereport.ncdu.zstd.inprogress /home/common/filereport.ncdu.zstd
