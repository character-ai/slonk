# Note: sudo required for chgrp on some filesystems
GROUP_NAME=${SLURM_GROUP:-"users"}
sudo chgrp ${GROUP_NAME} ~
chmod g+s ~
