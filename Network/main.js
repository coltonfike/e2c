var Web3 = require('web3');
var performance = require('perf_hooks')

const web3 = new Web3("ws://localhost:8505");
const myAddr = "0xe2ddab5e77df6d62f8661650e46d695be1963bf7";
const addresses = [
  "0x26519ea5fd73518efcf5ca13e6befab6836befce",
  "0xb7c420caaccc788a5542a54c22b653c525467d8c",
  "0x25b4c0b45f421e29a1660cfe3bb68511f7cfec26",
  "0x1c7e5ff787dc181bf5ce7f2eb38d582e0e246350",
  "0x8254b4b1fbe7272f7bf06c29b33cd4a2bbdf9db0"
];

// vars for functions
var nonce = 0;
var sent = 0;
var pending = 0;
var completedTxns = 0;
var completedContracts = 0;
var totalToSend = 10000;
var totalContractsToSend = 2000;
var accountLimit = 5000;
var txnHandlerComplete = false;
var contractHandlerComplete = false;

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

// generate index for the addresses, excluding the address that is running this node
function randAddr() {
  let r = Math.floor(Math.random() * 5);
  return addresses[r];
}

function send() {
  web3.eth.sendTransaction({
    from: myAddr,
    to: randAddr(),
    gas: 21000,
    nonce: nonce,
  }).on('error', function(error) {
    console.log(error)
  }).on('receipt', function() {
    pending--;
    completedTxns++;
    console.log("Completed: " + completedTxns);
  });
  console.log("Sent: " + nonce)
  nonce++;
  sent++;
}

function sendContract() {
  let abi = [{
    "inputs": [],
    "name": "hello",
    "outputs": [{
      "internalType": "string",
      "name": "",
      "type": "string"
    }],
    "stateMutability": "view",
    "type": "function"
  }];
  let code = '0x60806040526040518060400160405280600c81526020017f48656c6c6f20576f726c642100000000000000000000000000000000000000008152506000908051906020019061004f929190610062565b5034801561005c57600080fd5b50610166565b82805461006e90610105565b90600052602060002090601f01602090048101928261009057600085556100d7565b82601f106100a957805160ff19168380011785556100d7565b828001600101855582156100d7579182015b828111156100d65782518255916020019190600101906100bb565b5b5090506100e491906100e8565b5090565b5b808211156101015760008160009055506001016100e9565b5090565b6000600282049050600182168061011d57607f821691505b6020821081141561013157610130610137565b5b50919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052602260045260246000fd5b610232806101756000396000f3fe608060405234801561001057600080fd5b506004361061002b5760003560e01c806319ff1d2114610030575b600080fd5b61003861004e565b6040516100459190610119565b60405180910390f35b60606000805461005d9061018a565b80601f01602080910402602001604051908101604052809291908181526020018280546100899061018a565b80156100d65780601f106100ab576101008083540402835291602001916100d6565b820191906000526020600020905b8154815290600101906020018083116100b957829003601f168201915b5050505050905090565b60006100eb8261013b565b6100f58185610146565b9350610105818560208601610157565b61010e816101eb565b840191505092915050565b6000602082019050818103600083015261013381846100e0565b905092915050565b600081519050919050565b600082825260208201905092915050565b60005b8381101561017557808201518184015260208101905061015a565b83811115610184576000848401525b50505050565b600060028204905060018216806101a257607f821691505b602082108114156101b6576101b56101bc565b5b50919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052602260045260246000fd5b6000601f19601f830116905091905056fea264697066735822122018dc635da35edcf1fa54bc2ffdb277d23b2648496103691e84dd0a50ce4b7c2064736f6c63430008060033';
  let Contract = new web3.eth.Contract(abi, null, {
    data: code
  });

  Contract.deploy().send({
    from: myAddr,
    nonce: nonce,
    gas: 500000,
  }).on('receipt', function() {
    completedContracts++;
    pending--;
    //Contract.methods.hello().call({
    //  from: myAddr
    //})
  });
  nonce++;
}

async function txnHandler() {
  while (completedTxns < totalToSend) {
    pending++;
    send();
    await sleep(1);
  }
  txnHandlerComplete = true;
}

async function contractHandler() {
  // while (completedContracts < totalContractsToSend) {
  // pending++;
  // sendContract();
  // await sleep(5);
  // }
  contractHandlerComplete = true;
}

async function main() {
  console.time("Timer");
  txnHandler();
  contractHandler();
  while (!txnHandlerComplete || !contractHandlerComplete) {
    await sleep(100);
  }
  console.timeEnd("Timer");
  process.exit(0);
}

main();

// this is the code I that sends the transactions fast
/*
console.time("sentTimer");
console.time("Timer");
for (let i = 0; i < totalToSend; i++) {
  pending++;
  send();
}
console.timeEnd("sentTimer");
*/
