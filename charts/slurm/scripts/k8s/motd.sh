#!/bin/bash
source /etc/profile.d/k8s_env.sh
if [ -e /usr/games/lolcat ]
then
    if [[ "$(hostname)" == *login* ]]
    then
        PINK="\033[38;5;213m"
        GREEN="\033[38;5;154m"
        RESET="\033[0m"
        echo -e "${GREEN}                                ∙∙∙∙∙∙∙∙∙∙∙∙${RESET}"
        echo -e "${GREEN}                           ∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙${RESET}"
        echo -e "${GREEN}                       ∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙${RESET}"
        echo -e "${GREEN}                     ∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙${RESET}"
        echo -e "${PINK}        ▄▄▄▄▄▄  ${GREEN}  ∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙${RESET}"
        echo -e "${PINK}     ▄█████████▄${GREEN}∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙${PINK}▄▄${GREEN}∙∙∙∙∙∙∙${RESET}"
        echo -e "${PINK}    ▄█████  █████${GREEN}∙∙∙${PINK}▄██▌${GREEN}∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙${PINK}████${GREEN}∙∙∙∙${PINK}▄█▄${RESET}"
        echo -e "${PINK}   █████▀    ${GREEN}∙${PINK}▀▀${GREEN}∙∙∙${PINK}█████${GREEN}∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙${PINK}▀███${GREEN}∙∙∙${PINK}███▀${GREEN}∙${RESET}"
        echo -e "${PINK}   ███████  ${GREEN}∙∙∙∙∙∙∙∙${PINK}▐███${GREEN}∙∙∙∙∙∙${PINK}▄▄▄▄▄${GREEN}∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙${PINK}███${GREEN}∙∙${PINK}███▌${GREEN}∙∙∙${RESET}"
        echo -e "${PINK}   ▀█████████${GREEN}∙∙∙∙∙∙∙${PINK}▐███${GREEN}∙∙∙∙${PINK}▄███████▄${GREEN}∙∙∙${PINK}▄▄██▄▄██▄${GREEN}∙∙∙∙${PINK}███▄▄██▌${GREEN}∙∙∙∙∙${RESET}"
        echo -e "${PINK}    ▀██████████${GREEN}∙∙∙∙∙${PINK}▐███${GREEN}∙∙∙∙${PINK}██▌${GREEN}∙∙${PINK}▀███${GREEN}∙∙${PINK}▐█████████▌${GREEN}∙∙∙${PINK}████████▌${GREEN}∙∙∙∙${RESET}"
        echo -e "${PINK}      ▀█████████${GREEN}∙∙∙∙${PINK}▐███${GREEN}∙∙∙${PINK}▐██▌${GREEN}∙∙∙${PINK}███${GREEN}∙∙∙∙${PINK}███${GREEN}∙∙${PINK}▀███${GREEN}∙∙∙${PINK}███▀▀▀███${GREEN}∙∙∙∙∙${RESET}"
        echo -e "${PINK} ▄▄     ▀███████${GREEN}∙∙∙∙${PINK}▐███${GREEN}∙∙∙${PINK}▐██▌${GREEN}∙∙${PINK}▐███${GREEN}∙∙∙∙${PINK}███${GREEN}∙∙∙${PINK}███${GREEN}∙∙∙${PINK}███${GREEN}∙∙∙${PINK}███${GREEN}∙∙${PINK}▄▄████▄${RESET}"
        echo -e "${PINK}▐██▄     ▐██████${GREEN}∙∙∙∙${PINK}▐████${GREEN}∙∙${PINK}▐███▄████${GREEN}∙∙∙∙∙${PINK}███${GREEN}∙∙∙${PINK}███${GREEN}∙∙∙${PINK}███${GREEN}∙∙∙${PINK}███▄██████████▄${RESET}"
        echo -e "${PINK} ▀███▄  ▄██████▀${GREEN}∙∙∙∙${PINK}▐███▌${GREEN}∙∙∙${PINK}▀█████▀${GREEN}∙∙∙∙${PINK}▐████${GREEN}∙∙∙${PINK}███▌${GREEN}∙${PINK}▄███▄${GREEN}∙∙∙${PINK}▀███▀${GREEN}∙∙    ${PINK}███▄${RESET}"
        echo -e "${PINK}  ▀███████████▀${GREEN}∙∙∙∙∙∙${PINK}▀${GREEN}∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙     ${PINK}███${RESET}"
        echo -e "${PINK}    ▀██████▀${GREEN}∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙    ${PINK}████${RESET}"
        echo -e "${PINK}      ▀▀▀▀ ${GREEN} ∙∙∙∙∙∙${PINK}▄▄▄███████████████████████████▄▄▄▄${GREEN}∙∙∙∙∙∙∙∙∙∙∙∙∙∙   ${PINK}█████▀${RESET}"
        echo -e "${PINK}            ▄███████████████████████████████████████████████████████████▀${RESET}"
        echo -e "${PINK}         ▄██████████████████▀▀▀▀▀${GREEN}∙∙∙∙∙∙${PINK}▀▀▀▀███████████████████████████▀${RESET}"
        echo -e "${PINK}       ▄████████████████▀▀${GREEN}∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙${PINK}▀▀▀▀▀███████████▀▀${RESET}"
        echo -e "${PINK}      ▐████████████▀▀▀${GREEN}∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙${RESET}"
        echo -e "${PINK}       ▀██████▀▀▀${GREEN} ∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙${RESET}"
        echo -e "${PINK}         ▀▀▀▀     ${GREEN} ∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙${RESET}"
        echo -e "${GREEN}                       ∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙${RESET}"
        echo -e "${GREEN}                           ∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙∙${RESET}"
        echo -e "${GREEN}                                ∙∙∙∙∙∙∙∙∙∙∙∙∙∙${RESET}"
    else
        msg="$(hostname)"
        font=$(ls /usr/share/figlet/*.flf | shuf | head -n 1 | sed "s^.*/^^" | sed "s/.flf$//")
        ( /usr/bin/figlet -f $font "$msg" ) | /usr/games/lolcat -f -d 5 -F 0.4
    fi
    echo
fi
