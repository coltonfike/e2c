How to use:

running main.sh should run all the nodes and work correctly as long as the file path
for compiling is correct. I've left it unchanged so you will need to change it to 
work for you. Outputs from this script are in the logs folder

main.js is the script used to send the transactions
main.py is an alternate script that sends transactions

startNode.sh will run a node. It requires inputs for directory of the .ipc file, a port number, and a websocket port number
startClient.sh does the same as above, but runs without mining since it's the client node
watchNode.sh runs the watch command to observe outputs
init.sh will compile the code and reset the chain by deleting all the local files that store chain data

istanbul.sh does the same as main.sh but runs IBTF protocol

The json files are the genesis block data

The contract folder has code for the smart contract that is being deployed by main.js

The client folder and the node folders contain the node information (nodekey, .ipc file, static-nodes.json, ect)

The encode file has a script that will create the extra data field used in the json file for the e2c protocol

====================================================================================

If you want to use different nodes/add a node follow this tutorial (https://docs.goquorum.consensys.net/en/stable/Tutorials/Create-IBFT-Network)
**Important:** after creating the network as the tutorial lays out, we need to make the extra data field valid
for e2c. You need to run the script in the encode folder. Edit the script to contain the list of address for
the nodes you are using in the network. Do not include the client node, as all nodes listed will be considered
validators. In the json file, replace the extraData field with the output of that script

