#!/bin/bash

function error() {
  echo "ERROR: $1"
  exit 1
}

echo "Start simple"
./yay -S shfmt --noconfirm || error "unable to make shfmt"

./yay -Qsq shfmt || error "unable to install shfmt"

exit 0
