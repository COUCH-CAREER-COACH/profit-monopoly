from typing import Dict, Optional
import asyncio
import logging
from web3 import Web3
from eth_account import Account
from .core.strategy import MEVStrategy
from .core.mempool import MempoolMonitor
from .core.flashloan import FlashLoanManager
from .core.flashbots import FlashbotsManager

logger = logging.getLogger(__name__)

class MEVBot:
    def __init__(self, config_path: str):
        self.config = self._load_config(config_path)
        self.w3 = self._initialize_web3()
        self.account = Account.from_key(self.config['private_key'])
        
        # Initialize components
        self.strategy = MEVStrategy(self.w3, self.config)
        self.mempool = MempoolMonitor(self.w3, self.config)
        self.flashloan = FlashLoanManager(self.w3, self.config)
        self.flashbots = FlashbotsManager(
            self.w3,
            self.config['private_key'],
            self.config
        )
        
        # State tracking
        self.is_running = False
        self.current_block = 0
        self.pending_bundles = []
    
    def _load_config(self, config_path: str) -> Dict:
        """Load configuration from file"""
        import yaml
        with open(config_path, 'r') as f:
            return yaml.safe_load(f)
    
    def _initialize_web3(self) -> Web3:
        """Initialize Web3 with appropriate middleware"""
        w3 = Web3(Web3.HTTPProvider(self.config['rpc_url']))
        
        # Add necessary middleware
        w3.middleware_onion.add(
            self._build_flashbots_middleware()
        )
        
        return w3
    
    async def start(self):
        """Start the MEV bot"""
        logger.info("Starting MEV bot...")
        self.is_running = True
        
        try:
            # Start all components
            await self.mempool.start_monitoring()
            await self.flashloan.start_monitoring()
            
            # Start main loop
            await self._main_loop()
            
        except Exception as e:
            logger.error(f"Error in MEV bot: {e}")
            self.is_running = False
            raise
    
    async def stop(self):
        """Stop the MEV bot"""
        logger.info("Stopping MEV bot...")
        self.is_running = False
    
    async def _main_loop(self):
        """Main bot loop"""
        while self.is_running:
            try:
                # Update current block
                new_block = self.w3.eth.block_number
                if new_block > self.current_block:
                    await self._handle_new_block(new_block)
                    self.current_block = new_block
                
                # Process pending transactions
                await self._process_pending_transactions()
                
                # Small delay to prevent excessive CPU usage
                await asyncio.sleep(0.1)
                
            except Exception as e:
                logger.error(f"Error in main loop: {e}")
                await asyncio.sleep(1)
    
    async def _handle_new_block(self, block_number: int):
        """Handle new block events"""
        try:
            # Clean up old pending bundles
            self._cleanup_old_bundles(block_number)
            
            # Update market state
            await self._update_market_state()
            
            # Log block metrics
            self._log_block_metrics(block_number)
            
        except Exception as e:
            logger.error(f"Error handling new block {block_number}: {e}")
    
    async def _process_pending_transactions(self):
        """Process pending transactions for MEV opportunities"""
        try:
            # Get relevant pending transactions
            pending_txs = self.mempool.get_relevant_transactions()
            
            for tx in pending_txs:
                # Analyze for MEV opportunity
                opportunity = await self.strategy.analyze_opportunity(tx)
                
                if opportunity and opportunity.profit > self.config['min_profit']:
                    # Get flash loan if needed
                    if opportunity.requires_flash_loan:
                        loan_params = await self.flashloan.optimize_flash_loan(
                            opportunity.token,
                            opportunity.min_amount,
                            opportunity.max_amount,
                            opportunity.route
                        )
                        
                        if not loan_params:
                            continue
                    
                    # Create and submit bundle
                    bundle = await self._create_bundle(opportunity, loan_params)
                    if bundle:
                        await self._submit_bundle(bundle)
            
        except Exception as e:
            logger.error(f"Error processing pending transactions: {e}")
    
    async def _create_bundle(self, opportunity: Dict, loan_params: Optional[Dict]) -> Optional[Dict]:
        """Create a transaction bundle for the opportunity"""
        try:
            # Build transaction(s)
            transactions = []
            
            if loan_params:
                # Add flash loan transactions
                transactions.extend(
                    self._build_flash_loan_transactions(loan_params)
                )
            
            # Add MEV transaction
            transactions.append(
                self._build_mev_transaction(opportunity)
            )
            
            return {
                'transactions': transactions,
                'opportunity': opportunity,
                'loan_params': loan_params
            }
            
        except Exception as e:
            logger.error(f"Error creating bundle: {e}")
            return None
    
    async def _submit_bundle(self, bundle: Dict):
        """Submit bundle to Flashbots"""
        try:
            # Calculate target block
            target_block = self.current_block + 1
            
            # Submit to Flashbots
            bundle_hash = await self.flashbots.submit_bundle(
                bundle['transactions'],
                target_block
            )
            
            if bundle_hash:
                # Track pending bundle
                self.pending_bundles.append({
                    'hash': bundle_hash,
                    'target_block': target_block,
                    'bundle': bundle
                })
                
                logger.info(f"Submitted bundle {bundle_hash} for block {target_block}")
            
        except Exception as e:
            logger.error(f"Error submitting bundle: {e}")
    
    def _cleanup_old_bundles(self, current_block: int):
        """Clean up old pending bundles"""
        self.pending_bundles = [
            b for b in self.pending_bundles
            if b['target_block'] >= current_block - 2
        ]
    
    async def _update_market_state(self):
        """Update market state and parameters"""
        # Implement market state updates
        pass
    
    def _log_block_metrics(self, block_number: int):
        """Log metrics for the block"""
        metrics = {
            'block_number': block_number,
            'pending_bundles': len(self.pending_bundles),
            'gas_price': self.w3.eth.gas_price,
            'timestamp': self.w3.eth.get_block(block_number)['timestamp']
        }
        
        logger.info(f"Block metrics: {metrics}")
