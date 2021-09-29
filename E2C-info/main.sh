#!/bin/bash

let "NUM_NODES=$1-1"
DIR=$2
GENESIS=$3

# runs a node, needs the directory name, address, port number, http port number, and nodekey
run_node() {
  PRIVATE_CONFIG=ignore geth --datadir $DIR/$1 --verbosity 4 --nodiscover --syncmode full --mine --networkid 10 --port $2 --ws --ws.addr 'localhost' --ws.port $3 --ws.api admin,eth,miner,net,txpool,personal,web3 --allow-insecure-unlock > logs/logs_$1.txt 2>&1 &
}

run_client() {
  PRIVATE_CONFIG=ignore geth --datadir $DIR/$1 --verbosity 4 --nodiscover --syncmode full --networkid 10 --port $2 --ws --ws.addr 'localhost' --ws.port $3 --ws.api admin,eth,miner,net,txpool,personal,web3 --allow-insecure-unlock > logs/logs_$1.txt 2>&1 &
}

init() {
  geth --verbosity 0 --datadir $DIR/$1 init $2
}

rm -r $DIR/*/geth

cd ../cmd/geth
go install
cd ../../E2C-info

for i in $(seq 0 1 $NUM_NODES); do
  init $i $GENESIS
done
init client $GENESIS

port=30300
ws_port=8500
for i in $(seq 0 1 $NUM_NODES); do
  run_node $i $port $ws_port
  let "port++"
  let "ws_port++"
done

run_client client $port $ws_port
sleep 30
killall geth

