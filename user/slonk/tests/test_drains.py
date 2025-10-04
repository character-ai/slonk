"""'
File to assert slonk drain parsing is stable

Usage:
cd user/slonk && python -m pytest tests/test_drains.py
"""

import slonk.drains as drains
import unittest
import datetime

three_drain_fixtues = """NodeName=cluster-h100-9-78 Arch=x86_64 CoresPerSocket=1 
   CPUAlloc=0 CPUEfctv=96 CPUTot=96 CPULoad=1.15
   AvailableFeatures=gpu,h100,cluster-h100-9
   ActiveFeatures=gpu,h100,cluster-h100-9
   Gres=gpu:8
   NodeAddr=cluster-h100-9-78 NodeHostName=cluster-h100-9-78 Version=23.11.0-0rc1
   OS=Linux 5.15.133+ #1 SMP Wed Nov 8 17:30:28 UTC 2023 
   RealMemory=1048576 AllocMem=0 FreeMem=1803228 Sockets=96 Boards=1
   State=IDLE+DRAIN+MAINTENANCE+RESERVED ThreadsPerCore=1 TmpDisk=0 Weight=1 Owner=N/A MCS_label=N/A
   Partitions=low,general,high 
   BootTime=2023-12-22T11:11:17 SlurmdStartTime=2023-12-22T11:11:17
   LastBusyTime=2023-12-22T11:13:31 ResumeAfterTime=None
   CfgTRES=cpu=96,mem=1T,billing=96
   AllocTRES=
   CapWatts=n/a
   CurrentWatts=0 AveWatts=0
   ExtSensorsJoules=n/s ExtSensorsWatts=0 ExtSensorsTemp=n/s
   Reason=Prolog error: 571 xid 13 errors [root@2023-12-24T06:51:08]
   ReservationName=example

NodeName=cluster-h100-9-69 Arch=x86_64 CoresPerSocket=1 
   CPUAlloc=96 CPUEfctv=96 CPUTot=96 CPULoad=49.84
   AvailableFeatures=gpu,h100,cluster-h100-9
   ActiveFeatures=gpu,h100,cluster-h100-9
   Gres=gpu:8
   NodeAddr=cluster-h100-9-69 NodeHostName=cluster-h100-9-69 Version=23.11.0-0rc1
   OS=Linux 5.15.133+ #1 SMP Wed Nov 8 17:30:28 UTC 2023 
   RealMemory=1048576 AllocMem=0 FreeMem=855269 Sockets=96 Boards=1
   State=ALLOCATED+MAINTENANCE+RESERVED ThreadsPerCore=1 TmpDisk=0 Weight=1 Owner=N/A MCS_label=N/A
   Partitions=low,general,high 
   BootTime=2023-12-26T22:19:02 SlurmdStartTime=2023-12-26T22:19:02
   LastBusyTime=2023-12-26T22:18:31 ResumeAfterTime=None
   CfgTRES=cpu=96,mem=1T,billing=96
   AllocTRES=cpu=96
   CapWatts=n/a
   CurrentWatts=0 AveWatts=0
   ExtSensorsJoules=n/s ExtSensorsWatts=0 ExtSensorsTemp=n/s
   ReservationName=example

NodeName=cluster-h100-9-79 Arch=x86_64 CoresPerSocket=1 
   CPUAlloc=0 CPUEfctv=96 CPUTot=96 CPULoad=0.88
   AvailableFeatures=gpu,h100,cluster-h100-9
   ActiveFeatures=gpu,h100,cluster-h100-9
   Gres=gpu:8
   NodeAddr=cluster-h100-9-79 NodeHostName=cluster-h100-9-79 Version=23.11.0-0rc1
   OS=Linux 5.15.133+ #1 SMP Wed Nov 8 17:30:28 UTC 2023 
   RealMemory=1048576 AllocMem=0 FreeMem=1823144 Sockets=96 Boards=1
   State=IDLE+DRAIN+MAINTENANCE+RESERVED ThreadsPerCore=1 TmpDisk=0 Weight=1 Owner=N/A MCS_label=N/A
   Partitions=low,general,high 
   BootTime=2023-12-25T03:41:13 SlurmdStartTime=2023-12-25T03:41:13
   LastBusyTime=2023-12-25T03:40:43 ResumeAfterTime=None
   CfgTRES=cpu=96,mem=1T,billing=96
   AllocTRES=
   CapWatts=n/a
   CurrentWatts=0 AveWatts=0
   ExtSensorsJoules=n/s ExtSensorsWatts=0 ExtSensorsTemp=n/s
   Reason=SAM [root@2023-12-25T03:52:00]
   ReservationName=example

NodeName=cluster-h100-9-75 Arch=x86_64 CoresPerSocket=1 
   CPUAlloc=96 CPUEfctv=96 CPUTot=96 CPULoad=53.85
   AvailableFeatures=gpu,h100,cluster-h100-9
   ActiveFeatures=gpu,h100,cluster-h100-9
   Gres=gpu:8
   NodeAddr=cluster-h100-9-75 NodeHostName=cluster-h100-9-75 Version=23.11.0-0rc1
   OS=Linux 5.15.133+ #1 SMP Wed Nov 8 17:30:28 UTC 2023 
   RealMemory=1048576 AllocMem=0 FreeMem=837796 Sockets=96 Boards=1
   State=ALLOCATED+MAINTENANCE+RESERVED ThreadsPerCore=1 TmpDisk=0 Weight=1 Owner=N/A MCS_label=N/A
   Partitions=low,general,high 
   BootTime=2023-12-26T22:19:03 SlurmdStartTime=2023-12-26T22:19:03
   LastBusyTime=2023-12-26T22:18:33 ResumeAfterTime=None
   CfgTRES=cpu=96,mem=1T,billing=96
   AllocTRES=cpu=96
   CapWatts=n/a
   CurrentWatts=0 AveWatts=0
   ExtSensorsJoules=n/s ExtSensorsWatts=0 ExtSensorsTemp=n/s
   ReservationName=example
"""


class TestDrainFilters(unittest.TestCase):
    def test_get_drain_reasons(self):
        drain_reasons = drains.get_drain_reasons(three_drain_fixtues.splitlines())
        assert drain_reasons["cluster-h100-9-78"] == (
            "Prolog error: 571 xid 13 errors",
            datetime.datetime(2023, 12, 24, 6, 51, 8),
        )
        assert drain_reasons["cluster-h100-9-79"] == (
            "SAM",
            datetime.datetime(2023, 12, 25, 3, 52),
        )
        assert not "cluster-h100-9-69" in drain_reasons
        assert not "cluster-h100-9-75" in drain_reasons

    def test_valid_times_reasons(self):
        drain_reasons = drains.get_drain_reasons(three_drain_fixtues.splitlines())
        assert drains.is_valid_time(
            drain_reasons["cluster-h100-9-78"][1],
            start_time=datetime.datetime(2023, 12, 24, 3, 52),
            end_time=None,
        )
        assert not drains.is_valid_time(
            drain_reasons["cluster-h100-9-78"][1],
            start_time=datetime.datetime(2023, 12, 27, 3, 52),
            end_time=None,
        )
        assert drains.is_valid_time(
            drain_reasons["cluster-h100-9-78"][1],
            start_time=None,
            end_time=datetime.datetime(2023, 12, 27, 3, 52),
        )
        assert not drains.is_valid_time(
            drain_reasons["cluster-h100-9-78"][1],
            start_time=None,
            end_time=datetime.datetime(2023, 12, 22, 3, 52),
        )
        assert drains.is_valid_time(
            drain_reasons["cluster-h100-9-79"][1],
            start_time=datetime.datetime(2023, 12, 21, 3, 52),
            end_time=datetime.datetime(2023, 12, 27, 3, 52),
        )
        assert not drains.is_valid_time(
            drain_reasons["cluster-h100-9-79"][1],
            start_time=datetime.datetime(2023, 12, 21, 3, 52),
            end_time=datetime.datetime(2023, 12, 24, 3, 52),
        )
        assert not drains.is_valid_time(
            drain_reasons["cluster-h100-9-79"][1],
            start_time=datetime.datetime(2023, 12, 26, 3, 52),
            end_time=datetime.datetime(2023, 12, 27, 3, 52),
        )
