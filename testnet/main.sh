#!/bin/bash

# address of the nodes
node1_addr=0x87961E80677f0D5355C4e74b08A648707fcb0d5C
node2_addr=0xf5766FcbE23B650C576aC4c36D345072D7030366
node3_addr=0xb9e3d897f102aDb2bAF35d506BC1241075068838
node4_addr=0x853f6EeA65FE9dB51Cc5e300C0165be74d25A24b

# get the nodekey, changes each time init is called
get_key() {
  cd $1/geth
  key=$(cat nodekey)
  cd ../..
  echo $key
}

# builds the enode for the static-nodes file
get_enode() {
  key=$(bootnode -nodekeyhex $(get_key $1) -writeaddress)
  echo "\"enode://${key}@127.0.0.1:${2}?discport=0\""
}

# sets up the static-nodes.json file
generate_static_nodes() {
  enode1=$(get_enode node1 30311)
  enode2=$(get_enode node2 30312)
  enode3=$(get_enode node3 30313)
  enode4=$(get_enode node4 30314)

  echo -e "[\n${enode1},\n${enode2},\n${enode3},\n${enode4}\n]" > static-nodes.json
  cp -r static-nodes.json node1/
  cp -r static-nodes.json node2/
  cp -r static-nodes.json node3/
  cp -r static-nodes.json node4/
  rm static-nodes.json
}

# runs a node, needs the directory name, address, port number, http port number, and nodekey
run_node() {
  echo -e "$1 Output:\n" > logs/logs_$5.txt 
  geth --datadir $1/ --verbosity 0 --nodiscover --networkid 765567 --port $3 --mine --http --http.addr 'localhost' --http.port $4 --http.api admin,eth,miner,net,txpool,personal,web3 --allow-insecure-unlock --unlock $2 --password $1/password.txt >> logs/logs_$5.txt 2>&1
}

# sends 1000 small transactions from node4 to the value provided
send_transactions() {
  sleep .2
  for i in {1..1000}
  do
    echo "eth.sendTransaction({from:eth.coinbase, to:'$1', value:web3.toWei(0.05, 'ether'), gas:21000})" | geth attach node4/geth.ipc >> /dev/null 2>&1
    sleep .2
  done
}


# export the functions so parallel can call them
export -f run_node
export -f send_transactions

# remove the geth files so chain can be reset by init
rm -r */geth

# compile
cd ../cmd/geth
go install
cd ../bootnode
go install
cd ../../testnet

# init the genesis block
geth --verbosity 0 --datadir node1/ init e2c.json
geth --verbosity 0 --datadir node2/ init e2c.json
geth --verbosity 0 --datadir node3/ init e2c.json
geth --verbosity 0 --datadir node4/ init e2c.json

generate_static_nodes

# set up the arguments for run node
node1="node1 $node1_addr 30311 8501 $(get_key node1)"
node2="node2 $node2_addr 30312 8502 $(get_key node2)"
node3="node3 $node3_addr 30313 8503 $(get_key node3)"
node4="node4 $node4_addr 30314 8504 $(get_key node4)"

# export variables for parallel
export node1
export node2
export node3
export node4
export node1_addr

# run the 4 nodes and send transactions
# stores outputs in logs/log_[nodekey].txt
parallel --timeout 40 ::: 'run_node $node1' 'run_node $node2' 'run_node $node3' 'run_node $node4' 'send_transactions $node1_addr'
