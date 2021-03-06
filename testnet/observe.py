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
    run_time = time.time()
    while True:
        if time.time() - t > 30:
            txns_per_sec = float(txns) / (time.time() - t)
            print('Observed txns_per_sec: ' + str(txns_per_sec))
            return
        blocks = block_filter.get_new_entries()
        for block in blocks:
            txns += web3.eth.get_block_transaction_count(block)
            

        await asyncio.sleep(poll_interval)


def start(web3):
    loop = asyncio.new_event_loop()
    asyncio.set_event_loop(loop)
    try:
        loop.run_until_complete(asyncio.gather(block_handler(web3, .1)))
    finally:
        loop.close()


web3 = w3.Web3(w3.Web3.WebsocketProvider('ws://localhost:' + str(argv[1])))
start(web3)
