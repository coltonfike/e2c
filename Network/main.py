import asyncio
import concurrent.futures as cf
import threading
import time
from random import randint
from sys import argv

import web3 as w3


class Node:
    def __init__(self, addr, password, addresses, account_limit, to_send):

        self.web3 = w3.Web3(w3.Web3.WebsocketProvider('ws://localhost:8505'))

        self.my_addr = w3.Web3.toChecksumAddress(addr)
        self.web3.geth.personal.unlock_account(self.my_addr, password)

        self.addresses = addresses
        self.account_limit = account_limit
        self.nonce = 0
        self.to_send = to_send
        self.total = 0
        self.pending = []
        self.block_filter = self.web3.eth.filter('latest')

    def rand_addr(self):
        return self.addresses[randint(0, len(self.addresses) - 1)]

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
        while True:
            blocks = self.block_filter.get_new_entries()
            for block in blocks:
                for txn in self.pending:
                    try:
                        self.web3.eth.get_transaction(txn)
                        # if we get here, then the txn was found, so remove from pending
                        self.total += 1
                        self.pending.remove(txn)
                    except w3.exceptions.TransactionNotFound:
                        continue
                    except Exception as e:
                        print(e)
                        exit(1)
                if self.total >= self.to_send:
                    return
            await asyncio.sleep(poll_interval)

    async def run(self, poll_interval):
        while True:
            #  if len(self.pending) < self.account_limit:
            _ = self.send()
            self.nonce += 1
            #  self.pending.append(txn)
            # if this await isn't here, asyncio will never run block_handler
            await asyncio.sleep(poll_interval)
            #  if self.nonce >= self.to_send:
            #  return

    def start(self):
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        t1 = time.time()
        try:
            loop.run_until_complete(self.run(.0000001))
            #  loop.run_until_complete(
            #  asyncio.gather(self.block_handler(.1), self.run(.0000001)))
        finally:
            loop.close()
        return time.time() - t1


addresses = [
    w3.Web3.toChecksumAddress('0x26519ea5fd73518efcf5ca13e6befab6836befce'),
    w3.Web3.toChecksumAddress('0xb7c420caaccc788a5542a54c22b653c525467d8c'),
    w3.Web3.toChecksumAddress('0x25b4c0b45f421e29a1660cfe3bb68511f7cfec26'),
    w3.Web3.toChecksumAddress('0x1c7e5ff787dc181bf5ce7f2eb38d582e0e246350'),
    w3.Web3.toChecksumAddress('0x8254b4b1fbe7272f7bf06c29b33cd4a2bbdf9db0')
]

account_limit = int(argv[3])
to_send = int(argv[4])

node = Node(argv[1], argv[2], addresses, account_limit, to_send)
print(node.start())
