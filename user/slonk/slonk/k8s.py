#!/usr/bin/env python3

from datetime import datetime, timezone
import logging
import os
import re
import time
import warnings
from slonk.fingerprint import fingerprint

SLURM_JOBID_LABEL_KEY = "your-org/slurm-jobid"  # TODO: Replace with your organization
SLURM_GOAL_STATE_KEY = "slonk.your-org.com/slurm-goal-state"  # TODO: Replace with your organization domain

logger = logging.getLogger(__name__)

try:
    from kubernetes import client, config
    from kubernetes.client.rest import ApiException

    DISABLED = False
except ImportError:
    DISABLED = True


def protect(fn):
    def _protected(*args, **kwargs):
        global DISABLED
        if DISABLED:
            logger.warning("Not continuing, k8s client not installed")
            return
        create_kubernetes_configuration()
        warnings.filterwarnings("ignore", "Unverified HTTPS request")
        return fn(*args, **kwargs)

    return _protected


def create_kubernetes_configuration():
    api_server = "https://kubernetes.default.svc"
    token = open("/var/run/secrets/kubernetes.io/serviceaccount/token").read()
    # Create a configuration object
    configuration = client.Configuration()
    configuration.host = api_server
    configuration.verify_ssl = False
    configuration.api_key = {"authorization": f"Bearer {token}"}
    client.Configuration.set_default(configuration)


@protect
def update_node_taints(node_name, key, value=None, effect=None, untaint=False):
    v1 = client.CoreV1Api()
    max_retries = 3

    for attempt in range(max_retries):
        try:
            node = v1.read_node(name=node_name)

            if not node.spec.taints:
                node.spec.taints = []

            if untaint:
                # Remove the taint from the node
                node.spec.taints = [
                    taint for taint in node.spec.taints if taint.key != key
                ]
            else:
                # Add or update the taint on the node
                taint = client.V1Taint(effect=effect, key=key, value=value)
                existing_taint = next(
                    (t for t in node.spec.taints if t.key == key), None
                )
                if existing_taint:
                    existing_taint.effect = effect
                    existing_taint.value = value
                else:
                    node.spec.taints.append(taint)

            v1.replace_node(node_name, node)
            break
        except ApiException as e:
            if e.status == 409:  # Conflict error code
                print(f"Conflict detected on attempt {attempt + 1}. Retrying...")
                time.sleep(1)  # Wait a bit before retrying
            else:
                raise  # Re-raise the exception if it's not a conflict error
    else:
        raise Exception(
            f"Failed to update taint after {max_retries} attempts due to repeated conflicts."
        )


@protect
def add_node_label(node_name, key, value):
    v1 = client.CoreV1Api()
    node = v1.read_node(name=node_name)
    if node.metadata.labels is None:
        node.metadata.labels = {}
    node.metadata.labels[key] = value
    v1.patch_node(node_name, node)
    logger.info(f"Added label {key}={value} to {node_name}")


@protect
def add_node_annotation(node_name, key, value):
    v1 = client.CoreV1Api()
    node = v1.read_node(name=node_name)
    if node.metadata.annotations is None:
        node.metadata.annotations = {}
    node.metadata.annotations[key] = value
    v1.patch_node(node_name, node)
    logger.info(f"Added annotation {key}={value} to {node_name}")


@protect
def get_physical_node_name_from_node_annotations(node_name):
    # Get the physical node name from the node annotations.
    # There is a delay between the node being created and it being annotated, so this is less
    # reliable than using the fingerprint.
    v1 = client.CoreV1Api()
    # node = v1.read_node(name=node_name)
    node_data = v1.read_node(name=node_name, _preload_content=False)
    node = client.ApiClient().deserialize(node_data, "V1Node")
    if (
        node.metadata.annotations is None
        or "slonk.your-org.com/gpu-uuid-hash" not in node.metadata.annotations
    ):
        logger.info(f"Unable to identify underlying physical node for {node_name}")
        return False
    physical_node_name = node.metadata.annotations["slonk.your-org.com/gpu-uuid-hash"]
    return physical_node_name


@protect
def _get_physical_node(physical_node_name):
    # The physicalnode CRs should be in the same namespace as slurm pods.
    if "POD_NAMESPACE" in os.environ:
        namespace = os.environ["POD_NAMESPACE"]
    else:
        with open("/var/run/secrets/kubernetes.io/serviceaccount/namespace", "r") as f:
            namespace = f.read().strip()

    group = "slonk.your-org.com"
    version = "v1"
    plural = "physicalnodes"

    custom_api = client.CustomObjectsApi()
    return custom_api.get_namespaced_custom_object(
        group=group,
        version=version,
        namespace=namespace,
        plural=plural,
        name=physical_node_name,
    )


@protect
def _patch_physical_node_spec(physical_node_name, patch):
    # The physicalnode CRs should be in the same namespace as slurm pods.
    if "POD_NAMESPACE" in os.environ:
        namespace = os.environ["POD_NAMESPACE"]
    else:
        with open("/var/run/secrets/kubernetes.io/serviceaccount/namespace", "r") as f:
            namespace = f.read().strip()

    group = "slonk.your-org.com"
    version = "v1"
    plural = "physicalnodes"

    custom_api = client.CustomObjectsApi()
    custom_api.patch_namespaced_custom_object(
        group=group,
        version=version,
        namespace=namespace,
        plural=plural,
        name=physical_node_name,
        body=patch,
    )

@protect
def _patch_physical_node_status(physical_node_name, patch):
    # The physicalnode CRs should be in the same namespace as slurm pods.
    if "POD_NAMESPACE" in os.environ:
        namespace = os.environ["POD_NAMESPACE"]
    else:
        with open("/var/run/secrets/kubernetes.io/serviceaccount/namespace", "r") as f:
            namespace = f.read().strip()

    group = "slonk.your-org.com"
    version = "v1"
    plural = "physicalnodes"

    custom_api = client.CustomObjectsApi()
    custom_api.patch_namespaced_custom_object_status(
        group=group,
        version=version,
        namespace=namespace,
        plural=plural,
        name=physical_node_name,
        body=patch,
    )


@protect
def get_physical_node_spec(physical_node_name):
    try:
        physical_node = _get_physical_node(physical_node_name)
    except ApiException as e:
        logger.error(f"Failed to get physical node {physical_node_name}: {e}")
        return None
    if not physical_node:
        return None

    return physical_node.get("spec", {})


@protect
def update_physical_node_slurm_goal_state(
    physical_node_name, goal_state, manual=True, reason=None
):
    # Set the goal state of the physical node.
    # Doesn't update the reason filed in the CRD.
    if not reason:
        patch = {
            "spec": {
                "slurmNodeSpec": {
                    "goalState": goal_state,
                },
                "manual": manual,
            }
        }
    else:
        patch = {
            "spec": {
                "slurmNodeSpec": {
                    "goalState": goal_state,
                    "reason": reason,
                },
                "manual": manual,
            }
        }
    try:
        _patch_physical_node_spec(physical_node_name, patch)
        logger.info(
            f"Updated physical node {physical_node_name} goal state to {goal_state}"
        )
    except ApiException as e:
        logger.error(f"Failed to update physical node {physical_node_name}: {e}")
        raise


@protect
def patch_physical_node_condition(physical_node_name, condition_type, reason, message):
    try:
        physical_node = _get_physical_node(physical_node_name)
    except ApiException as e:
        logger.error(f"Failed to get physical node {physical_node_name}: {e}")
        return

    conditions = physical_node.get("status", {}).get("slurmNodeConditions", {})

    # Update or add the condition
    curr_time = datetime.now(timezone.utc).replace(microsecond=0)
    curr_time_iso = curr_time.strftime("%Y-%m-%dT%H:%M:%SZ")
    conditions[condition_type] = {
        "type": condition_type,
        "status": "True",
        "lastTransitionTime": curr_time_iso,
        "reason": reason,
        "message": message,
    }

    patch = {"status": {"slurmNodeConditions": conditions}}

    # Apply the patch
    try:
        _patch_physical_node_status(physical_node_name, patch)
    except ApiException as e:
        logger.error(f"Failed to patch physical node {physical_node_name}: {e}")
        return

    logger.info(
        f"Updated physical node {physical_node_name} condition {condition_type}, reason: {reason}, message: {message}"
    )


@protect
def taint_node(node_name, key, value=None, effect=None):
    if value:
        # can't be longer than 63 chars
        value = value[:63]
        value = re.sub(r"\W", "-", value)
    retval = update_node_taints(
        node_name, key, value=value, effect=effect, untaint=False
    )
    logger.info(f"Added taint {key}={value}:{effect} to {node_name}")
    return retval


@protect
def untaint_node(node_name, key):
    retval = update_node_taints(node_name, key, untaint=True)
    logger.info(f"Removed taint {key} on {node_name}")
    return retval


@protect
def list_nodes():
    v1 = client.CoreV1Api()
    nodes = v1.list_node()
    return nodes.items


@protect
def get_node_from_pod(pod_name, namespace):
    v1 = client.CoreV1Api()
    pod = v1.read_namespaced_pod(name=pod_name, namespace=namespace)
    return v1.read_node(name=pod.spec.node_name)


def get_my_node():
    pod_name, namespace = _get_pod_name_and_namespace()
    return get_node_from_pod(pod_name, namespace)


def getnodename():
    if "K8S_NODE_NAME" in os.environ:
        return os.environ["K8S_NODE_NAME"]
    return get_my_node().metadata.name


@protect
def drain_node(node_name):
    v1 = client.CoreV1Api()
    core_v1_api = client.CoreV1Api()

    # Cordon the node
    body = {"spec": {"unschedulable": True}}
    core_v1_api.patch_node(node_name, body)

    # Get all pods on the node
    pods = core_v1_api.list_pod_for_all_namespaces(
        field_selector=f"spec.nodeName={node_name},status.phase!=Succeeded,status.phase!=Failed"
    )

    # Evict each pod
    for pod in pods.items:
        try:
            eviction = client.V1beta1Eviction(
                metadata=client.V1ObjectMeta(
                    name=pod.metadata.name, namespace=pod.metadata.namespace
                )
            )
            core_v1_api.create_namespaced_pod_eviction(
                name=pod.metadata.name, namespace=pod.metadata.namespace, body=eviction
            )
        except ApiException as e:
            if e.status != 409:  # Ignore error if pod is already being evicted
                raise


def _get_pod_name_and_namespace():
    if "POD_NAMESPACE" in os.environ:
        pod_namespace = os.environ["POD_NAMESPACE"]
    else:
        with open("/var/run/secrets/kubernetes.io/serviceaccount/namespace", "r") as f:
            pod_namespace = f.read().strip()

    if "POD_NAME" in os.environ:
        pod_name = os.environ["POD_NAME"]
    elif "K8S_POD_NAME" in os.environ:
        pod_name = os.environ["K8S_POD_NAME"]
    else:
        # pod name is likely the hostname
        pod_name = os.uname()[1]

    return pod_name, pod_namespace


@protect
def get_pod_deletion_cost():
    """
    Get the current deletion cost of this pod.
    Returns the deletion cost if set, otherwise None.
    """
    pod_name, namespace = _get_pod_name_and_namespace()

    v1 = client.CoreV1Api()
    try:
        # Read the pod
        pod = v1.read_namespaced_pod(name=pod_name, namespace=namespace)
        # Get the deletion cost from the annotations
        annotations = pod.metadata.annotations
        if annotations is None:
            return 0
        deletion_cost = annotations.get("controller.kubernetes.io/pod-deletion-cost")
        return int(deletion_cost) if deletion_cost is not None else 0
    except ApiException as e:
        logger.error(
            "Failed to get pod deletion cost for %s in %s: %s", pod_name, namespace, e
        )
        return 0


@protect
def update_pod_deletion_cost(deletion_cost):
    """
    Update the deletion cost of this pod. Pod with lower deletion cost is usually
    evicted first.
    """
    pod_name, namespace = _get_pod_name_and_namespace()

    v1 = client.CoreV1Api()
    body = {
        "metadata": {
            "annotations": {
                "controller.kubernetes.io/pod-deletion-cost": str(deletion_cost)
            }
        }
    }
    try:
        # Patch the pod with the new deletion cost
        v1.patch_namespaced_pod(name=pod_name, namespace=namespace, body=body)
        logger.debug(
            "Updated pod deletion cost for %s in %s to %d",
            pod_name,
            namespace,
            deletion_cost,
        )
    except ApiException as e:
        logger.error(
            "Failed to update pod deletion cost for %s in %s: %s",
            pod_name,
            namespace,
            e,
        )


@protect
def pod_slurm_jobid_label_exists() -> bool:
    """
    Check if the pod label 'c.ai/slurm-jobid' exists.
    Returns True if the label exists, otherwise False.
    """
    pod_name, namespace = _get_pod_name_and_namespace()

    v1 = client.CoreV1Api()
    try:
        pod = v1.read_namespaced_pod(name=pod_name, namespace=namespace)
        labels = pod.metadata.labels
        return SLURM_JOBID_LABEL_KEY in labels
    except ApiException as e:
        logger.error("Failed to check label for %s in %s: %s", pod_name, namespace, e)
        return False


@protect
def add_pod_label_slurm_jobid(job_id: str):
    """
    Add or update the pod label specified by SLURM_JOBID_LABEL_KEY with the provided job_id.
    """
    pod_name, namespace = _get_pod_name_and_namespace()

    v1 = client.CoreV1Api()
    body = {"metadata": {"labels": {SLURM_JOBID_LABEL_KEY: job_id}}}
    try:
        # Patch the pod with the new label
        v1.patch_namespaced_pod(name=pod_name, namespace=namespace, body=body)
        logger.debug(
            "Updated pod label '%s' for %s in %s to %s",
            SLURM_JOBID_LABEL_KEY,
            pod_name,
            namespace,
            job_id,
        )
    except ApiException as e:
        logger.error(
            "Failed to update pod label '%s' for %s in %s: %s",
            SLURM_JOBID_LABEL_KEY,
            pod_name,
            namespace,
            e,
        )


@protect
def remove_pod_label_slurm_jobid():
    """
    Remove the pod label specified by SLURM_JOBID_LABEL_KEY.
    """
    pod_name, namespace = _get_pod_name_and_namespace()

    v1 = client.CoreV1Api()
    body = {"metadata": {"labels": {SLURM_JOBID_LABEL_KEY: None}}}
    try:
        # Patch the pod to remove the label
        v1.patch_namespaced_pod(name=pod_name, namespace=namespace, body=body)
        logger.debug(
            "Removed label '%s' from pod %s in namespace %s",
            SLURM_JOBID_LABEL_KEY,
            pod_name,
            namespace,
        )
    except ApiException as e:
        logger.error(
            "Failed to remove label '%s' from pod %s in namespace %s: %s",
            SLURM_JOBID_LABEL_KEY,
            pod_name,
            namespace,
            e,
        )


@protect
def check_node_goal_state_okay() -> bool:
    """
    Check if the slurm goal state annotation is being respected
    Returns True if the node is supposed to be up, otherwise False.
    """
    try:
        physical_node = _get_physical_node(fingerprint())
    except ApiException as e:
        logger.error(
            f"Failed to get physical node {fingerprint()}: {e}. Assuming okay."
        )
        return True

    if not physical_node:
        logger.error(f"Physical node {fingerprint()} not found. Assuming okay.")
        return True

    return (
        physical_node.get("spec", {}).get("slurmNodeSpec", {}).get("goalState")
        != "down"
    )


@protect
def node_slurm_jobid_label_exists() -> bool:
    """
    Check if the node label 'c.ai/slurm-jobid' exists.
    Returns True if the label exists, otherwise False.
    """
    node_name = getnodename()
    if not node_name:
        logger.error("Failed to get node name when checking node label")
        return False

    v1 = client.CoreV1Api()
    try:
        node = v1.read_node(name=node_name)
        return SLURM_JOBID_LABEL_KEY in node.metadata.labels
    except ApiException as e:
        logger.error("Failed to check label for %s: %s", node_name, e)
        return False


@protect
def add_node_label_slurm_jobid(job_id: str):
    """
    Label the current Kubernetes node with the slurm job id.
    """
    node_name = getnodename()
    if not node_name:
        logger.error("Failed to get node name when trying to label node")
        return

    try:
        add_node_label(node_name, SLURM_JOBID_LABEL_KEY, job_id)
    except client.ApiException as e:
        logger.error(
            "Failed to update node label '%s' for %s: %s",
            SLURM_JOBID_LABEL_KEY,
            node_name,
            e,
        )


@protect
def remove_node_label_slurm_jobid():
    """
    Remove slurm job id label from node.
    """
    node_name = getnodename()
    if not node_name:
        logger.error("Failed to get node name when trying to remove node label")
        return

    try:
        add_node_label(node_name, SLURM_JOBID_LABEL_KEY, None)
    except client.ApiException as e:
        logger.error(
            "Failed to remove label '%s' from node %s: %s",
            SLURM_JOBID_LABEL_KEY,
            node_name,
            e,
        )


if __name__ == "__main__":
    print(list_nodes())
