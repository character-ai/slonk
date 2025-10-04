#!/usr/bin/env python3


import importlib
import pkgutil
import random
import re

from slonk.utils import bash

try:
    import terminaltexteffects

    TEXT_EFFECTS = True
except Importerror:
    TEXT_EFFECTS = False

# 7-bit C1 ANSI sequences
ANSI_ESCAPE_RE = re.compile(r"\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])")


def fortune():
    return ANSI_ESCAPE_RE.sub("", bash("fortune"))


def logo():
    return ANSI_ESCAPE_RE.sub("", bash("/etc/update-motd.d/00-slonk"))


def list_submodules(package_name):
    try:
        package = __import__(package_name, fromlist=[""])
    except ImportError:
        print(f"Package '{package_name}' not found.")
        return []

    submodules = []
    for loader, module_name, is_pkg in pkgutil.walk_packages(
        package.__path__, package.__name__ + "."
    ):
        submodules.append(module_name)
    return submodules


def get_all_effects():
    effects = []
    for module in list_submodules("terminaltexteffects.effects"):
        pkg = importlib.import_module(module)
        effect, args = pkg.get_effect_and_args()
        effects.append(effect)
    return effects


def setup_args(parser):
    pass


def main(args):
    logo_text = logo()
    fortune_text = fortune()
    text = logo_text + "\n\n" + fortune_text
    if TEXT_EFFECTS:
        effect_class = random.choice(get_all_effects())
        effect = effect_class(text)
        with effect.terminal_output(end_symbol="\n") as terminal:
            for frame in effect:
                terminal.print(frame)
    else:
        print(text)


if __name__ == "__main__":
    main()
