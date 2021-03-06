import asyncio
import concurrent.futures as cf
import threading
import time
from random import randint
from sys import argv
from web3.middleware import geth_poa_middleware

import web3 as w3


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
        self.latency = 0
        self.pending = {}
        self.block_filter = self.web3.eth.filter('latest')

    def rand_addr(self):
        r = randint(0, len(self.addresses) - 2)
        if r >= self.index:
            r += 1
        return self.addresses[r]

    def send(self):
        to_addr = self.rand_addr()
        from_addr = self.my_addr
        return self.web3.eth.send_transaction({
            'to': to_addr,
            'from': from_addr,
            'value': 100000,
            'gas': 21000,
            'nonce': self.nonce
        })

    async def block_handler(self, poll_interval):
        t = time.time()
        while True:
            blocks = self.block_filter.get_new_entries()
            for block in blocks:
                b = self.web3.eth.get_block(block)
                for txn in b['transactions']:
                    try:
                        self.latency += time.time() - self.pending[bytes(txn)]
                        self.total += 1
                        self.pending.pop(txn)
                    except KeyError:
                        # transaction wasn't sent by this node
                        continue
                    except Exception as e:
                        print(e)
                        exit(1)
            if time.time() - t > 30:
                print('Average latency: ' +
                      str(self.latency / self.total) + ' seconds')
                return
            await asyncio.sleep(poll_interval)

    async def run(self, poll_interval):
        t = time.time()
        while True:
            if time.time() - t > 30:
                return
            txn = self.send()
            self.nonce += 1
            self.pending[txn] = time.time()
            await asyncio.sleep(poll_interval)

    def start(self):
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        t1 = time.time()
        try:
            if self.index == 1:
                loop.run_until_complete(
                    asyncio.gather(self.block_handler(.1), self.run(.0000001)))
            else:
                loop.run_until_complete(self.run(.0000001))
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
