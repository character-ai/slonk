#!/usr/bin/env python3

"""
Syncs the ldap cache and post-processes it to apply some custom changes.
"""

import logging
import subprocess
import sys
import json
import time
import os
import contextlib

GITHUB_APP_ID = int(os.environ.get("GITHUB_APP_ID", "0"))
GITHUB_ORG = os.environ.get("GITHUB_ORG", "your-org")
LDAP_FOLDER = "/home/common/.ldap"
NSSCACHE = "/usr/local/bin/nsscache"

logging.basicConfig(
    format="%(asctime)s | %(levelname)s | %(name)s:%(lineno)d | %(message)s",
    stream=sys.stdout,
    level="INFO",
)

logger = logging.getLogger("ldap_sync")


@contextlib.contextmanager
def open_sot(name, mode="r"):
    if mode == "w":
        name_ = name + ".cache"
    else:
        maybe_name = name + ".cache_raw"
        if os.path.exists(os.path.join(LDAP_FOLDER, maybe_name)):
            name_ = maybe_name
        else:
            name_ = name + ".cache"
    yield open(os.path.join(LDAP_FOLDER, name_), mode)


@contextlib.contextmanager
def atomic_write(name):
    path = os.path.join(LDAP_FOLDER, name + ".cache")
    f = open(path + ".tmp", "w")
    yield f
    f.close()
    os.rename(path + ".tmp", path)


# keep consistent uid and homedir in case user changes
historic_uid = {}
if os.path.exists("/home/common/.ldap/passwd.cache_raw"):
    read_filename = "/home/common/.ldap/passwd.cache_raw"
else:
    read_filename = "/home/common/.ldap/passwd.cache"
with open_sot("passwd") as f:
    for line in f:
        line = line.strip()
        if not line:
            continue
        username, passwd, uid, gid, _blank, homedir, shell = line.split(":")
        assert uid == gid
        historic_uid[username] = uid


# perform the nss cache update
logger.info("Updating nsscache")
nss_output = subprocess.check_output(f"{NSSCACHE} update --full", shell=True)
logger.info(f"{NSSCACHE} output: {nss_output}")

# update everyone's shell to be zsh needed
logger.info("reading groups")
with open_sot("group") as f:
    for line in f:
        if line.startswith("zsh:"):
            zsh_members = set(line.strip().split(":")[-1].split(","))
            break
    else:
        raise RuntimeError("No zsh group found")

output = []
logger.info("reading users")

new_gid = {}
with open_sot("passwd") as f:
    for line in f:
        line = line.strip()
        if not line:
            continue
        username, passwd, uid, gid, _blank, homedir, shell = line.split(":")

        ldaps_uid = uid

        # keep old values if possible
        uid = gid = historic_uid.get(username, uid)
        new_gid[ldaps_uid] = uid
        homedir = "/home/" + username
        if username in zsh_members:
            shell = shell.replace("/bin/bash", "/bin/zsh")
        line = ":".join([username, passwd, uid, gid, _blank, homedir, shell])
        output.append(line)

output = sorted(output)

logger.info("writing users")
with atomic_write("passwd") as f:
    for item in output:
        f.write(f"{item}\n")

output = []
with open_sot("group") as f:
    for line in f:
        line = line.strip()
        if not line:
            continue
        group, star, gid, users = line.split(":")
        if group in historic_uid:
            gid = historic_uid[group]  # tie user and group ids
        output.append(":".join([group, star, gid, users]))
with atomic_write("group") as f:
    for item in output:
        f.write(f"{item}\n")

with open_sot("shadow") as f:
    data = f.read()
with atomic_write("shadow") as f:
    f.write(data)
