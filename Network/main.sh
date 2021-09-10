#!/bin/bash

# runs a node, needs the directory name, address, port number, http port number, and nodekey
run_node() {
  echo -e "$1 Output:\n" > logs/logs_$1.txt 
  PRIVATE_CONFIG=ignore geth --datadir $1 --verbosity 4 --nodiscover --syncmode full --mine --networkid 765567 --port $2 --ws --ws.addr 'localhost' --ws.port $3 --ws.api admin,eth,miner,net,txpool,personal,web3 --allow-insecure-unlock >> logs/logs_$1.txt 2>&1
}

run_client() {
  echo -e "$1 Output:\n" > logs/logs_$1.txt 
  PRIVATE_CONFIG=ignore geth --datadir $1 --verbosity 4 --nodiscover --syncmode full --networkid 765567 --port $2 --ws --ws.addr 'localhost' --ws.port $3 --ws.api admin,eth,miner,net,txpool,personal,web3 --allow-insecure-unlock >> logs/logs_$1.txt 2>&1
}

rm -r */geth

cd ../quorum/cmd/geth
go install
cd ../../../Network

geth --verbosity 0 --datadir node1 init e2c.json
geth --verbosity 0 --datadir node2 init e2c.json
geth --verbosity 0 --datadir node3 init e2c.json
geth --verbosity 0 --datadir node4 init e2c.json
geth --verbosity 0 --datadir client init e2c.json

# export the functions so parallel can call them
export -f run_node
export -f run_client

# set up the arguments for run node
node1="node1 30300 8501"
node2="node2 30301 8502"
node3="node3 30302 8503"
node4="node4 30303 8504"
client="client 30304 8505"

# export variables for parallel
export node1
export node2
export node3
export node4
export client

parallel ::: 'run_node $node1' 'run_node $node2' 'run_node $node3' 'run_node $node4' 'run_client $client'
