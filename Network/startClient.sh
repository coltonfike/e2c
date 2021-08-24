#!/bin/bash
 PRIVATE_CONFIG=ignore geth --datadir $1 --verbosity 3 --nodiscover --syncmode full --networkid 765567 --port $2 --ws --ws.addr 'localhost' --ws.port $3 --ws.api admin,eth,miner,net,txpool,personal,web3 --allow-insecure-unlock --unlock "0xe2ddab5e77df6d62f8661650e46d695be1963bf7" --password client/password.txt
