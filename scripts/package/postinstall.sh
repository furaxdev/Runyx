#!/bin/sh
# Runs once after `dpkg -i`/`pacman -U` finishes copying files -- a
# .deb/Arch package has no GUI of its own (dpkg/pacman are command-line
# tools), so this is the closest thing to a "personalized setup
# screen" one can give a package install: a branded terminal welcome
# message confirming what just happened and how to actually start
# Runyx, in the same navy/blue the Windows and macOS installers use
# (ANSI 256-color approximations of #1B2233/#6F8FCB/#F4EFE6, drawn
# directly as colored block characters rather than converting an image
# -- there's no terminal image protocol every user's terminal
# supports, but every terminal supports ANSI color).
set -eu

if [ -t 1 ]; then
  NAVY='\033[48;5;17m'
  BLUE='\033[38;5;110m'
  CREAM='\033[38;5;230m'
  RESET='\033[0m'
  BOLD='\033[1m'
else
  NAVY=''; BLUE=''; CREAM=''; RESET=''; BOLD=''
fi

printf '\n'
printf "${BLUE}  ███${RESET}\n"
printf "${BLUE} █████${RESET}\n"
printf "${BLUE}██${CREAM}█${BLUE}██${CREAM}█${BLUE}██${RESET}\n"
printf "${BLUE}████████${RESET}\n"
printf "${BLUE}█${RESET}  ${BLUE}██${RESET}  ${BLUE}█${RESET}\n"
printf '\n'
printf "${BOLD}Runyx is installed.${RESET}\n"
printf 'Run it from your application menu, or from a terminal:\n\n'
printf "  ${BOLD}runyx${RESET}            open the app window\n"
printf "  ${BOLD}runyx doctor${RESET}     check what this machine can run\n"
printf "  ${BOLD}runyx serve${RESET}      run the daemon with no window\n\n"
printf 'Fully local once a model is downloaded -- no account, no telemetry.\n\n'
