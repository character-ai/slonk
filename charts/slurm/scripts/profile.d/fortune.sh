#!/bin/bash
if [ ! -z "$PS1" ] && [ ! -e ~/.hushlogin ] && command -v fortune >/dev/null
then
    fortune "$(cat ~/.fortune 2>/dev/null || true)"
fi
