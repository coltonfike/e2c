# E2C

E2C is an alternative consensus protocol built on go-quorum. E2C provides more transactions/second when compared to the IBFT protocol while also allowing more fault nodes.

### Setup
Nodes should be generated using the guide by [Consensys for the IBFT protocol](https://docs.goquorum.consensys.net/en/stable/Tutorials/Private-Network/Create-IBFT-Network/). Next, you need to copy the address of all the nodes in the validator and paste them into the array in this [script](https://github.com/coltonfike/quorum/blob/master/E2C-info/encode/encode.go). The output of this script will the extra data and mix hash. In the genesis.json, you need to replace the extra data and mix hash with the outputs of the script. Finally, you need to change the consensus engine from istanbul to e2c. You can replace the istanbul field with the following:
`"e2c": {
            "delta": 200,
            "blockSize": 200
        },`
Delta should be set to the upper bound on Network speed, in ***milliseconds***. Block size specifies how many transactions should be included in the block. There are other values that can be changed in the genesis file, but it's not recommended to modify these.

Client nodes should be generate as well. Client nodes are nodes not in the validator set. These nodes observe the state of the externally and will always contain the correct state. All web3 clients should connect to these nodes.

### Testing

Some test data has already been supplied. `main.sh` will run nodes for 30 seconds and redirect their output to the logs directory. It's usage is as follows:
`./main.sh [numNodes] [directory to node files] [path/to/genesis.json]`

`send_transactions.sh` opens multiple web3 connections to the client node and sends transactions over the network.
