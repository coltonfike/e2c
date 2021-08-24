import asyncio
import concurrent.futures as cf
import threading
import time
from random import randint
from sys import argv

import web3 as w3


class Node:
    def __init__(self, node, addresses, account_limit, to_send):

        if node == 'node1':
            self.web3 = w3.Web3(
                w3.Web3.WebsocketProvider('ws://localhost:8501'))
            self.my_addr = 0
        elif node == 'node2':
            self.web3 = w3.Web3(
                w3.Web3.WebsocketProvider('ws://localhost:8502'))
            self.my_addr = 1
        elif node == 'node3':
            self.web3 = w3.Web3(
                w3.Web3.WebsocketProvider('ws://localhost:8503'))
            self.my_addr = 2
        elif node == 'node4':
            self.web3 = w3.Web3(
                w3.Web3.WebsocketProvider('ws://localhost:8504'))
            self.my_addr = 3

        self.addresses = addresses
        self.account_limit = account_limit
        self.nonce = 0
        self.to_send = to_send
        self.total = 0
        self.pending = []
        self.block_filter = self.web3.eth.filter('latest')
        self.lock = threading.Lock()

    def rand_addr(self):
        r = randint(0, len(self.addresses) - 2)
        if r >= self.my_addr:
            r += 1
        return r

    def send(self):
        to_addr = w3.Web3.toChecksumAddress(self.addresses[self.rand_addr()])
        from_addr = w3.Web3.toChecksumAddress(self.addresses[self.my_addr])
        return self.web3.eth.send_transaction({
            'to': to_addr,
            'from': from_addr,
            'value': 100000,
            'gas': 21000,
            'nonce': self.nonce
        })

    async def block_handler(self, poll_interval):
        while True:
            with self.lock:
                blocks = self.block_filter.get_new_entries()
            for block in blocks:
                for txn in self.pending:
                    try:
                        self.web3.eth.get_transaction(txn)
                        # if we get here, then the txn was found, so remove from pending
                        self.total += 1
                        with self.lock:
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
            if len(self.pending) < self.account_limit:
                with self.lock:
                    txn = self.send()
                self.nonce += 1
                with self.lock:
                    self.pending.append(txn)
            # if this await isn't here, asyncio will never run block_handler
            await asyncio.sleep(poll_interval)
            if self.nonce >= self.to_send:
                return

    def start(self):
        print(str(self.my_addr) + ' Started')
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        t1 = time.time()
        try:
            loop.run_until_complete(
                asyncio.gather(self.block_handler(.1), self.run(.0000001)))
        finally:
            loop.close()
        return time.time() - t1


addresses = [
    '0x26519ea5fd73518efcf5ca13e6befab6836befce',
    '0xb7c420caaccc788a5542a54c22b653c525467d8c',
    '0x25b4c0b45f421e29a1660cfe3bb68511f7cfec26',
    '0x1c7e5ff787dc181bf5ce7f2eb38d582e0e246350',
    '0x8254b4b1fbe7272f7bf06c29b33cd4a2bbdf9db0'
]

account_limit = int(argv[1])
to_send = int(argv[2])

nodes = [
    Node('node1', addresses, account_limit, to_send),
    Node('node2', addresses, account_limit, to_send),
    Node('node3', addresses, account_limit, to_send),
    Node('node4', addresses, account_limit, to_send)
]

with cf.ThreadPoolExecutor() as executor:
    threads = [executor.submit(node.start) for node in nodes]

print(max([thread.result() for thread in threads]))
