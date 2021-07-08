#!/bin/bash

function say() {
    echo
    echo "---------------------------------------------------------------"
    echo $1
    echo "---------------------------------------------------------------"
    echo
}


if [ "$EUID" -ne 0 ]
  then say "Use sudo to run this script"
  exit
fi

for FILE in $(find ../examples/preflight/host/*.yaml)
do
    if [ "${FILE}" = "../examples/preflight/host/tcp-load-balancer.yaml" ]
        then say "Skipping ${FILE}"
	continue
    fi

    say "Running Test ${FILE}"

    ../bin/preflight --interactive=false  $FILE 
done


