#!/usr/bin/env python3

import os
import time
import json
from web3 import Web3
from dotenv import load_dotenv
import logging
from datetime import datetime

# Setup logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler('bot_monitor.log'),
        logging.StreamHandler()
    ]
)
logger = logging.getLogger('MEVBotMonitor')

# Load environment variables
load_dotenv()

class MEVBotMonitor:
    def __init__(self):
        self.w3 = Web3(Web3.HTTPProvider(os.getenv('HTTPS_URL_SEPOLIA')))
        self.bot_address = os.getenv('BOT_ADDRESS')
        self.bot_abi = self._load_abi()
        self.contract = self.w3.eth.contract(
            address=self.bot_address,
            abi=self.bot_abi
        )
        
        # Metrics
        self.metrics = {
            'total_transactions': 0,
            'successful_arbitrages': 0,
            'failed_transactions': 0,
            'total_profit': 0,
            'gas_spent': 0,
            'last_block_number': 0
        }
    
    def _load_abi(self):
        with open('artifacts/contracts/V2ArbBot.sol/V2ArbBot.json') as f:
            contract_json = json.load(f)
            return contract_json['abi']
    
    def check_health(self):
        """Check basic health metrics of the bot"""
        try:
            is_shutdown = self.contract.functions.isShutdown().call()
            calls_this_block = self.contract.functions.callsThisBlock().call()
            current_block = self.w3.eth.block_number
            
            logger.info(f"""
            Bot Health Check:
            - Shutdown Status: {'🔴 Shutdown' if is_shutdown else '🟢 Active'}
            - Calls This Block: {calls_this_block}/3
            - Current Block: {current_block}
            """)
            
            return not is_shutdown
        except Exception as e:
            logger.error(f"Health check failed: {str(e)}")
            return False
    
    def monitor_events(self, from_block):
        """Monitor and log relevant events"""
        try:
            events = self.contract.events.ArbitrageExecuted.get_logs(
                fromBlock=from_block
            )
            
            for event in events:
                self.metrics['successful_arbitrages'] += 1
                self.metrics['total_profit'] += event['args']['profit']
                
                logger.info(f"""
                Arbitrage Executed:
                - Token In: {event['args']['tokenIn']}
                - Token Out: {event['args']['tokenOut']}
                - Amount: {event['args']['amount']}
                - Profit: {event['args']['profit']}
                """)
        
        except Exception as e:
            logger.error(f"Event monitoring failed: {str(e)}")
    
    def run(self):
        """Main monitoring loop"""
        logger.info("Starting MEV Bot Monitor")
        
        while True:
            try:
                # Check bot health
                if not self.check_health():
                    logger.warning("Bot is not healthy!")
                
                # Monitor new events
                current_block = self.w3.eth.block_number
                if current_block > self.metrics['last_block_number']:
                    self.monitor_events(self.metrics['last_block_number'])
                    self.metrics['last_block_number'] = current_block
                
                # Log metrics every hour
                if datetime.now().minute == 0:
                    logger.info(f"Hourly Metrics: {json.dumps(self.metrics, indent=2)}")
                
                time.sleep(12)  # Wait for new block
                
            except Exception as e:
                logger.error(f"Monitoring error: {str(e)}")
                time.sleep(60)  # Wait longer on error

if __name__ == "__main__":
    monitor = MEVBotMonitor()
    monitor.run()
