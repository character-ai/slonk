#!/bin/bash
C=~/.config/gcloud/configurations/config_default 
if [ -e $C ]
then
    # use has set up gcloud before
    grep "disable_file_logging = True" $C > /dev/null 
    if [ "$?" != "0" ]
    then
        # user does not have logging disabled
        echo "SLONK FYI: Turning off gcloud logging."
        gcloud config set core/disable_file_logging True
    fi
fi
