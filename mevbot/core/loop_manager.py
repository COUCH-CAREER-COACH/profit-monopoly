"""Main loop manager for MEV bot."""
import os
import time
import asyncio
import logging
from typing import Dict, List, Optional, Set, Any
from web3 import Web3
from eth_typing import HexStr

from mevbot.core.safety.coordinator import SafetyCoordinator, EmergencyLevel
from mevbot.core.strategy_base import BaseStrategy

logger = logging.getLogger(__name__)

class LoopManager:
    """
    Manages the main execution loop of the MEV bot.
    
    Features:
    - Strategy execution
    - Safety system integration
    - Transaction management
    - Performance monitoring
    - Error recovery
    """
    
    def __init__(self, config: Dict, web3: Web3):
        """Initialize loop manager with configuration."""
        self.config = config
        self.web3 = web3
        self.running = False
        self.strategies: Dict[str, BaseStrategy] = {}
        
        # Initialize safety system
        self.safety = SafetyCoordinator(config.get('safety', {}), web3)
        
        # Track active transactions
        self.pending_transactions: Set[str] = set()
        self.tx_receipts: Dict[str, Dict] = {}
        
        # Performance tracking
        self.start_time = None
        self.total_executions = 0
        self.successful_executions = 0
        
        # Monitoring
        self.last_health_check = time.time()
        self.health_check_interval = config.get('health_check_interval', 60)
        
    async def start(self) -> None:
        """Start the main execution loop."""
        if self.running:
            logger.warning("Loop manager already running")
            return
        
        try:
            logger.info("Starting loop manager")
            self.running = True
            self.start_time = time.time()
            
            # Start safety system
            await self.safety.start()
            
            # Start strategies
            for strategy in self.strategies.values():
                await strategy.start()
            
            # Start monitoring tasks
            asyncio.create_task(self._monitor_transactions())
            asyncio.create_task(self._monitor_health())
            
            # Start main loop
            await self._run_loop()
            
        except Exception as e:
            logger.error(f"Error starting loop manager: {str(e)}")
            await self.safety.record_incident(
                EmergencyLevel.CRITICAL,
                f"Loop manager start failed: {str(e)}",
                "LoopManager"
            )
            raise
    
    async def stop(self) -> None:
        """Stop the main execution loop."""
        if not self.running:
            return
        
        logger.info("Stopping loop manager")
        self.running = False
        
        try:
            # Stop all strategies
            for strategy in self.strategies.values():
                await strategy.stop()
            
            # Stop safety system
            await self.safety.stop()
            
            # Log performance metrics
            self._log_performance_metrics()
            
        except Exception as e:
            logger.error(f"Error stopping loop manager: {str(e)}")
    
    async def add_strategy(self, strategy: BaseStrategy) -> None:
        """Add a new strategy to the loop manager."""
        strategy_id = strategy.get_id()
        if strategy_id in self.strategies:
            raise ValueError(f"Strategy {strategy_id} already exists")
        
        self.strategies[strategy_id] = strategy
        if self.running:
            await strategy.start()
        logger.info(f"Added strategy: {strategy_id}")
    
    async def remove_strategy(self, strategy_id: str) -> None:
        """Remove a strategy from the loop manager."""
        if strategy_id not in self.strategies:
            raise ValueError(f"Strategy {strategy_id} not found")
        
        strategy = self.strategies[strategy_id]
        await strategy.stop()
        del self.strategies[strategy_id]
        logger.info(f"Removed strategy: {strategy_id}")
    
    async def _run_loop(self) -> None:
        """Run the main execution loop."""
        while self.running:
            try:
                # Execute strategies in parallel
                strategy_tasks = []
                for strategy in self.strategies.values():
                    if await self._should_execute_strategy(strategy):
                        strategy_tasks.append(
                            self._execute_strategy_safely(strategy)
                        )
                
                if strategy_tasks:
                    await asyncio.gather(*strategy_tasks, return_exceptions=True)
                
                # Brief pause between iterations
                await asyncio.sleep(0.1)
                
            except Exception as e:
                logger.error(f"Error in main loop: {str(e)}")
                await self.safety.record_incident(
                    EmergencyLevel.WARNING,
                    f"Main loop error: {str(e)}",
                    "LoopManager"
                )
                await asyncio.sleep(1)
    
    async def _should_execute_strategy(self, strategy: BaseStrategy) -> bool:
        """Check if a strategy should be executed."""
        try:
            # Check if strategy is ready
            if not strategy.is_ready():
                return False
            
            # Check network conditions
            if not await self.safety.check_network_conditions():
                return False
            
            # Check system resources
            if not await self.safety.check_system_resources():
                return False
            
            return True
            
        except Exception as e:
            logger.error(f"Error checking strategy execution: {str(e)}")
            return False
    
    async def _execute_strategy_safely(self, strategy: BaseStrategy) -> None:
        """Execute a strategy with safety checks."""
        strategy_id = strategy.get_id()
        
        try:
            # Register strategy execution
            await self.safety.register_strategy(strategy_id)
            
            # Execute strategy
            self.total_executions += 1
            result = await strategy.execute()
            
            # Validate transaction if returned
            if result and isinstance(result, dict):
                if await self.safety.validate_transaction(result):
                    # Submit transaction
                    tx_hash = await self._submit_transaction(result)
                    if tx_hash:
                        self.pending_transactions.add(tx_hash)
                        self.successful_executions += 1
            
        except Exception as e:
            logger.error(f"Error executing strategy {strategy_id}: {str(e)}")
            await self.safety.record_incident(
                EmergencyLevel.WARNING,
                f"Strategy execution error: {str(e)}",
                strategy_id
            )
            raise
            
        finally:
            # Unregister strategy
            await self.safety.unregister_strategy(strategy_id)
    
    async def _submit_transaction(self, tx: Dict) -> Optional[str]:
        """Submit a transaction with safety checks."""
        try:
            # Final safety validation
            if not await self.safety.validate_transaction(tx):
                return None
            
            # Sign and send transaction
            signed_tx = await self._sign_transaction(tx)
            tx_hash = await self.web3.eth.send_raw_transaction(signed_tx.rawTransaction)
            tx_hash_hex = HexStr(tx_hash.hex())
            
            logger.info(f"Submitted transaction: {tx_hash_hex}")
            return tx_hash_hex
            
        except Exception as e:
            logger.error(f"Error submitting transaction: {str(e)}")
            return None
    
    async def _sign_transaction(self, tx: Dict) -> Any:
        """Sign a transaction using the secure key manager."""
        try:
            # Get private key from secure storage
            private_key = await self.safety.key_manager.get_private_key(
                self.config['key_id'],
                self.config['key_password']
            )
            
            if not private_key:
                raise ValueError("Failed to retrieve private key")
            
            # Sign transaction
            return self.web3.eth.account.sign_transaction(tx, private_key)
            
        except Exception as e:
            logger.error(f"Error signing transaction: {str(e)}")
            raise
    
    async def _monitor_transactions(self) -> None:
        """Monitor pending transactions."""
        while self.running:
            try:
                completed_txs = set()
                
                for tx_hash in self.pending_transactions:
                    try:
                        receipt = await self.web3.eth.get_transaction_receipt(tx_hash)
                        if receipt:
                            completed_txs.add(tx_hash)
                            self.tx_receipts[tx_hash] = receipt
                            
                            # Log success/failure
                            if receipt['status'] == 1:
                                logger.info(f"Transaction successful: {tx_hash}")
                            else:
                                logger.warning(f"Transaction failed: {tx_hash}")
                                
                    except Exception as e:
                        logger.error(f"Error checking transaction {tx_hash}: {str(e)}")
                
                # Remove completed transactions
                self.pending_transactions -= completed_txs
                
                # Brief pause
                await asyncio.sleep(1)
                
            except Exception as e:
                logger.error(f"Error in transaction monitor: {str(e)}")
                await asyncio.sleep(5)
    
    async def _monitor_health(self) -> None:
        """Monitor system health."""
        while self.running:
            try:
                current_time = time.time()
                
                # Run health check at interval
                if current_time - self.last_health_check >= self.health_check_interval:
                    await self.check_system_health()
                    self.last_health_check = current_time
                
                # Brief pause
                await asyncio.sleep(1)
                
            except Exception as e:
                logger.error(f"Error in health monitor: {str(e)}")
                await asyncio.sleep(5)
    
    async def monitor_transaction(self, tx_hash: str) -> None:
        """Monitor a specific transaction."""
        try:
            receipt = await self.web3.eth.wait_for_transaction_receipt(tx_hash)
            if receipt:
                self.tx_receipts[tx_hash] = receipt
                # Log success/failure
                if receipt['status'] == 1:
                    logger.info(f"Transaction successful: {tx_hash}")
                else:
                    logger.warning(f"Transaction failed: {tx_hash}")
        except Exception as e:
            logger.error(f"Error monitoring transaction {tx_hash}: {str(e)}")
            raise

    def get_active_strategies(self) -> List[str]:
        """Get list of active strategy IDs."""
        return list(self.strategies.keys())

    async def execute_strategy(self, strategy_id: str) -> None:
        """Execute a specific strategy."""
        if strategy_id not in self.strategies:
            raise ValueError(f"Strategy {strategy_id} not found")
        
        strategy = self.strategies[strategy_id]
        await self._execute_strategy_safely(strategy)

    async def check_system_health(self) -> Dict:
        """Check system health status."""
        try:
            import psutil
            
            # Get system metrics
            cpu_usage = psutil.cpu_percent()
            memory = psutil.virtual_memory()
            
            # Determine health status
            healthy = cpu_usage <= 80.0 and memory.percent <= 90.0
            
            system_health = {
                'cpu_usage': cpu_usage,
                'memory_usage': memory.percent,
                'healthy': healthy
            }
            
            return system_health
            
        except Exception as e:
            logger.error(f"Error checking system health: {str(e)}")
            return {
                'cpu_usage': 100,
                'memory_usage': 100,
                'healthy': False,
                'error': str(e)
            }

    def _log_performance_metrics(self) -> None:
        """Log performance metrics."""
        if not self.start_time:
            return
        
        runtime = time.time() - self.start_time
        success_rate = (
            (self.successful_executions / self.total_executions * 100)
            if self.total_executions > 0 else 0
        )
        
        logger.info(
            f"Performance Metrics:\n"
            f"Runtime: {runtime:.2f} seconds\n"
            f"Total Executions: {self.total_executions}\n"
            f"Successful Executions: {self.successful_executions}\n"
            f"Success Rate: {success_rate:.2f}%\n"
            f"Pending Transactions: {len(self.pending_transactions)}\n"
            f"Completed Transactions: {len(self.tx_receipts)}"
        )
