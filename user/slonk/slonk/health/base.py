#!/usr/bin/env python3

import logging
import socket
from subprocess import CalledProcessError

import slonk.k8s
from slonk.utils import get_cluster, is_tcpx_cluster, is_onprem_cluster, bash
import slonk.utils

logger = logging.getLogger(__name__)


class HealthCheckError(Exception):
    def __init__(
        self,
        condition_type: str,
        reason: str,
        message: str,
        remediation: str = None,
    ):
        self.condition_type = condition_type
        self.reason = reason
        self.message = message
        self.remediation = remediation
        super().__init__(reason)

    def handle_mitigation(self):
        return False


class NeedsPodRestart(HealthCheckError):
    TAINT = "slonk.your-org.com/action-quit"

    def __init__(self, condition_type, reason, message):
        super().__init__(condition_type, reason, message, remediation="pod_restart")

    # TODO: In some clusters, sometimes a slurm node would think its hostname is the k8s node name,
    # instead of the pod name. Restart node to get around that.
    def handle_mitigation(self):
        logger.info(
            "Needs pod restart. Cluster: %s. IsOnPrem: %s. IsTCPX: %s",
            get_cluster(),
            is_onprem_cluster(),
            is_tcpx_cluster(),
        )

        if is_onprem_cluster():
            logger.info("Directly restarting pod through scontrol")
            bash(f"scontrol reboot asap {socket.gethostname()}", sudo=True)
        else:
            if not is_tcpx_cluster():
                logger.warning("Unknown cluster type. Assuming it's a tcpx cluster.")
            slonk.k8s.taint_node(
                slonk.k8s.getnodename(), self.TAINT, self.reason, "NoSchedule"
            )

        return False


class NeedsPowerCycle(HealthCheckError):
    TAINT = "slonk.your-org.com/action-reboot"

    def __init__(self, condition_type, reason, message):
        super().__init__(condition_type, reason, message, remediation="reboot")

    def handle_mitigation(self):
        logger.info(
            "Needs node power cycle. Cluster: %s. IsOnPrem: %s. IsTCPX: %s",
            get_cluster(),
            is_onprem_cluster(),
            is_tcpx_cluster(),
        )

        if is_onprem_cluster():
            logger.info("Directly restarting node through ipmitool")
            bash("ipmitool power cycle", sudo=True)
        else:
            if not is_tcpx_cluster():
                logger.warning("Unknown cluster type. Assuming it's a tcpx cluster.")
            slonk.k8s.taint_node(
                slonk.k8s.getnodename(), self.TAINT, self.reason, "NoSchedule"
            )

        return False


class NeedsManual(HealthCheckError):
    TAINT = "slonk.your-org.com/action-manual"
    LEGACY_TAINT = "needs_manual_intervention"

    def __init__(self, condition_type, reason, message):
        super().__init__(condition_type, reason, message, remediation="manual")

    def handle_mitigation(self):
        logger.info(
            "Needs manual fix. Cluster: %s. IsOnPrem: %s. IsTCPX: %s",
            get_cluster(),
            is_onprem_cluster(),
            is_tcpx_cluster(),
        )

        slonk.k8s.taint_node(
            slonk.k8s.getnodename(), self.TAINT, self.reason, "NoSchedule"
        )
        slonk.k8s.taint_node(
            slonk.k8s.getnodename(), self.LEGACY_TAINT, self.reason, "NoSchedule"
        )

        return False


class NeedsRMA(HealthCheckError):
    TAINT = "slonk.your-org.com/action-rma"
    LEGACY_TAINT = "needs_rma"

    def __init__(self, condition_type, reason, message):
        super().__init__(condition_type, reason, message, remediation="rma")

    def handle_mitigation(self):
        logger.info(
            "Needs RMA. Cluster: %s. IsOnPrem: %s. IsTCPX: %s",
            get_cluster(),
            is_onprem_cluster(),
            is_tcpx_cluster(),
        )

        slonk.k8s.taint_node(
            slonk.k8s.getnodename(), self.TAINT, self.reason, "NoSchedule"
        )
        slonk.k8s.taint_node(
            slonk.k8s.getnodename(), self.LEGACY_TAINT, self.reason, "NoSchedule"
        )

        return False


class NeedsMaintenance(HealthCheckError):
    LABEL = "cloud.google.com/perform-maintenance"
    VALUE = "true"

    # TODO: use reason
    def __init__(self, reason, message):
        super().__init__("MaintenanceEvent", reason, message, remediation="taint")

    def handle_mitigation(self):
        logger.info(
            "Performing maintenance. Cluster: %s. IsOnPrem: %s. IsTCPX: %s",
            get_cluster(),
            is_onprem_cluster(),
            is_tcpx_cluster(),
        )

        if not is_tcpx_cluster():
            logger.warning("No mitigation for non-tcpx clusters")
            return False

        slonk.k8s.add_node_label(
            slonk.k8s.getnodename(),
            self.LABEL,
            self.VALUE,
        )
        slonk.k8s.taint_node(
            slonk.k8s.getnodename(),
            self.LABEL,
            self.VALUE,
            "NoSchedule",
        )
        return False
