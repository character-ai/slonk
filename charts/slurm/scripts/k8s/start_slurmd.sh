#!/bin/bash
set -ex

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
SLURM_DIR=$SCRIPT_DIR/slurm
SLURM_NAME=${K8S_POD_NAME:-$(hostname)}

PHYSICAL_NODE_NAME=$(slonk fingerprint)
SLURM_GOAL_STATE=""
SLURM_REASON=""
if [ -n "$PHYSICAL_NODE_NAME" ] && [ "$PHYSICAL_NODE_NAME" != "unknonwn" ] && [ "${ENABLE_PHYSICAL_NODE_GOALSTATE:-false}" = "true" ]; then
    # Initialize start time and timeout
    START_TIME=$(date +%s)
    TIMEOUT=60
    TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
    APISERVER=https://kubernetes.default.svc
    while [ $(($(date +%s) - START_TIME)) -lt $TIMEOUT ]; do
    API_DOMAIN=${SLONK_API_DOMAIN:-"your-org.com"}
    SLURM_GOAL_STATE=$(curl -k -X GET $APISERVER/apis/slonk.${API_DOMAIN}/v1/namespaces/slurm/physicalnodes/$PHYSICAL_NODE_NAME \
        -H "Authorization: Bearer $TOKEN" \
        -H 'Accept: application/json' | jq -r '.spec.slurmNodeSpec.goalState')

    if [ -z "$SLURM_GOAL_STATE" ] || [ "$SLURM_GOAL_STATE" = "null" ]; then
        echo "Error obtaining slurm goal state or slurm goal state is null. Sleeping for 10 seconds."
        sleep 10
    else
        echo "Slurm goal state is $SLURM_GOAL_STATE."
        SLURM_REASON=$(curl -k -X GET $APISERVER/apis/slonk.${API_DOMAIN}/v1/namespaces/slurm/physicalnodes/$PHYSICAL_NODE_NAME \
            -H "Authorization: Bearer $TOKEN" \
            -H 'Accept: application/json' | jq -r '.spec.slurmNodeSpec.reason')
        break
    fi
    done
fi

if [ -z "$SLURM_GOAL_STATE" ] || [ "$SLURM_GOAL_STATE" = "null" ]; then
  echo "Error obtaining slurm goal state or slurm goal state is null. Assuming up."
  SLURM_GOAL_STATE="up"
fi

# apply a sequence of state changes to clear any existing flags.
scontrol update NodeName=${SLURM_NAME} state=down reason="Init error: placeholder" comment=
if [ $SLURM_GOAL_STATE = "init" ]; then
  echo "Slurm goal state is init. Holding node in FUTURE state..."
  SLURM_REASON="Init error: goal state is init. ${SLURM_REASON}"
  scontrol update NodeName=${SLURM_NAME} state=future reason="${SLURM_REASON}"
elif [ $SLURM_GOAL_STATE = "up" ]; then
  # the final RESUME state will set the node to IDLE and NOT_RESPONDING
  # the NOT_RESPONDING flag will be cleared once slurmd starts
  echo "Slurm goal state is up. Resuming node..."
  scontrol update NodeName=${SLURM_NAME} state=future
  scontrol update NodeName=${SLURM_NAME} state=resume
  if ! /usr/local/bin/slonk health --mode all; then
    echo "Health check failed, draining the node"
    SLURM_REASON="Init error: failed health check. ${SLURM_REASON}"
    scontrol update NodeName=${SLURM_NAME} state=future
    scontrol update NodeName=${SLURM_NAME} state=drain reason="${SLURM_REASON}"
  fi
elif [ $SLURM_GOAL_STATE = "drain" ]; then
  echo "Slurm goal state is drain. Draining node..."
  SLURM_REASON="Init error: goal state is drain. ${SLURM_REASON}"
  scontrol update NodeName=${SLURM_NAME} state=future
  scontrol update NodeName=${SLURM_NAME} state=drain reason="${SLURM_REASON}"
elif [ $SLURM_GOAL_STATE = "down" ]; then
  echo "Slurm goal state is down. Exiting..."
  SLURM_REASON="Init error: goal state is down. ${SLURM_REASON}"
  scontrol update NodeName=${SLURM_NAME} state=down reason="${SLURM_REASON}"
  sleep 30
  exit 1
else
  echo "Unknown slurm goal state: $SLURM_GOAL_STATE. Assuming it should be up. Resuming node..."
  scontrol update NodeName=${SLURM_NAME} state=future
  scontrol update NodeName=${SLURM_NAME} state=resume
fi

# update node IP address
scontrol update NodeName=${SLURM_NAME} NodeAddr=${SLURM_NAME}


# wait 30 seconds for DNS to catch up (probably)
sleep 30

# start slurmd
service slurmd start

# Set reason again
if [ -n "$SLURM_REASON" ]; then
  sleep 30
  scontrol update NodeName=${SLURM_NAME} reason="${SLURM_REASON}"
fi
