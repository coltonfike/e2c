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

cd testnet

# remove data from previous run
rm -r $DIR/*/geth

# initialize the nodes with new genesis file
for i in $(seq 0 1 $NUM_NODES); do
  init $i $GENESIS
done
init client $GENESIS

# start the nodes
port=30300
ws_port=8500
for i in $(seq 0 1 $NUM_NODES); do
  run_node $i $port $ws_port
  let "port++"
  let "ws_port++"
done

run_client client $port $ws_port

# wait a few seconds for them to get started
sleep 10

# start the client accounts to send transactions
python3 main.py '0xe2ddab5e77df6d62f8661650e46d695be1963bf7' 'client' 10000 10000 $ws_port &
python3 main.py '0xd18aefd325d127fe3e1d6272180a8629413ddc6b' 'password' 10000 10000 $ws_port &
python3 main.py '0xcf7d7b22af30aadce47930cd234ed34c4488da5e' 'password' 10000 10000 $ws_port &
python3 main.py '0x82aa48615b89237a0195441da44a63dcbf199f21' 'password' 10000 10000 $ws_port &
python3 main.py '0x12c825237c38cfe2f879fcd475cb438ed0778d8e' 'password' 10000 10000 $ws_port &
python3 main.py '0xdee5bc6e1c404c693c0fcf145dcfcb64330eb8bd' 'password' 10000 10000 $ws_port &
python3 main.py '0xec317a80394abb23c8940b2b7f2d66e0e3c97677' 'password' 10000 10000 $ws_port &
python3 main.py '0xb48bd20a8c8e687511e36df039c17b8704c2c115' 'password' 10000 10000 $ws_port &

# run a script to observe the chain to get txns/second
python3 observe.py $ws_port

# shouldn't need to kill these, but just in case
pkill 'python3 main.py'

# breifly wait for the python scripts to end so we don't get errors
sleep 1

# kill all the running nodes
killall geth
