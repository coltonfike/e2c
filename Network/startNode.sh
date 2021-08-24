#!/bin/bash
PRIVATE_CONFIG=ignore geth --datadir $1 --verbosity 3 --nodiscover --syncmode full --mine --networkid 765567 --port $2 --ws --ws.addr 'localhost' --ws.port $3 --ws.api admin,eth,miner,net,txpool,personal,web3 --allow-insecure-unlock
