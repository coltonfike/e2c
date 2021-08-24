#!/bin/bash

rm -r */geth

cd ../quorum/cmd/geth
go install
cd ../../../Network

geth --verbosity 0 --datadir node1 init e2c.json
geth --verbosity 0 --datadir node2 init e2c.json
geth --verbosity 0 --datadir node3 init e2c.json
geth --verbosity 0 --datadir node4 init e2c.json
geth --verbosity 0 --datadir client init e2c.json
