import asyncio
import time
from random import randint
from sys import argv

import web3 as w3
from web3.middleware import geth_poa_middleware
from solcx import compile_source


class Node:
    def __init__(self, addr, password, addresses, account_limit, to_send,
                 port):

        self.web3 = w3.Web3(
            w3.Web3.WebsocketProvider('ws://localhost:' + str(port)))

        self.my_addr = w3.Web3.toChecksumAddress(addr)
        self.web3.geth.personal.unlock_account(self.my_addr, password)
        self.index = addresses.index(self.my_addr)
        self.web3.middleware_onion.inject(geth_poa_middleware, layer=0)

        self.addresses = addresses
        self.account_limit = account_limit
        self.nonce = 0
        self.to_send = to_send
        self.total = 0
        self.pending = []
        self.block_filter = self.web3.eth.filter('latest')

        compiled_sol = compile_source(
            '''
            pragma solidity >0.5.0;

                contract Greeter {
                string public greeting;

                constructor() public {
                    greeting = 'Hello';
                }

                function setGreeting(string memory _greeting) public {
                    greeting = _greeting;
                }

                function greet() view public returns (string memory) {
                    return greeting;
                }
            }
            ''',
            output_values=['abi', 'bin']
        )

        contract_id, contract_interface = compiled_sol.popitem()
        bytecode = contract_interface['bin']
        abi = contract_interface['abi']

        Greeter = self.web3.eth.contract(abi=abi, bytecode=bytecode)
        tx_hash = Greeter.constructor().transact({
            'from': self.my_addr,
            'gas': 1000000,
            'nonce': self.nonce
        })
        self.nonce += 1
        tx_receipt = self.web3.eth.wait_for_transaction_receipt(tx_hash)
        self.greeter = self.web3.eth.contract(
            address=tx_receipt.contractAddress, abi=abi)

    def rand_addr(self):
        r = randint(0, len(self.addresses) - 2)
        if r >= self.index:
            r += 1
        return self.addresses[r]

    def send(self):
        return self.greeter.functions.greet().call({'from': self.my_addr})

    async def run(self, poll_interval):
        t = time.time()
        while True:
            if time.time() - t > 30:
                return
            _ = self.send()
            self.nonce += 1
            await asyncio.sleep(poll_interval)

    def start(self):
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        t1 = time.time()
        try:
            loop.run_until_complete(self.run(1))
        finally:
            loop.close()
        return time.time() - t1


addresses = [
    w3.Web3.toChecksumAddress('0xe2ddab5e77df6d62f8661650e46d695be1963bf7'),
    w3.Web3.toChecksumAddress('0xd18aefd325d127fe3e1d6272180a8629413ddc6b'),
    w3.Web3.toChecksumAddress('0xcf7d7b22af30aadce47930cd234ed34c4488da5e'),
    w3.Web3.toChecksumAddress('0x82aa48615b89237a0195441da44a63dcbf199f21'),
    w3.Web3.toChecksumAddress('0x12c825237c38cfe2f879fcd475cb438ed0778d8e'),
    w3.Web3.toChecksumAddress('0xdee5bc6e1c404c693c0fcf145dcfcb64330eb8bd'),
    w3.Web3.toChecksumAddress('0xec317a80394abb23c8940b2b7f2d66e0e3c97677'),
    w3.Web3.toChecksumAddress('0xb48bd20a8c8e687511e36df039c17b8704c2c115'),
]

account_limit = int(argv[3])
to_send = int(argv[4])

node = Node(argv[1], argv[2], addresses, account_limit, to_send, argv[5])
node.start()
