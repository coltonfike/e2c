import asyncio
import concurrent.futures as cf
import threading
import time
from random import randint
from sys import argv

import web3 as w3


async def block_handler(web3, poll_interval):
    block_filter = web3.eth.filter('latest')
    txns = 0
    txns_per_sec = 0
    t = time.time()
    while True:
        blocks = block_filter.get_new_entries()
        for block in blocks:
            txns += web3.eth.get_block_transaction_count(block)
            txns_per_sec = float(txns) / (time.time() - t)
            print(txns_per_sec)

        await asyncio.sleep(poll_interval)


def start(web3):
    loop = asyncio.new_event_loop()
    asyncio.set_event_loop(loop)
    try:
        loop.run_until_complete(asyncio.gather(block_handler(web3, .1)))
    finally:
        loop.close()


web3 = w3.Web3(w3.Web3.WebsocketProvider('ws://localhost:8505'))
start(web3)
