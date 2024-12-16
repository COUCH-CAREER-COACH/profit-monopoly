"""Mempool monitoring and transaction management"""
from typing import Dict, List, Optional, Set
from dataclasses import dataclass
from web3 import Web3
import asyncio
import logging
from eth_typing import Address, HexStr
from concurrent.futures import ThreadPoolExecutor
import time

logger = logging.getLogger(__name__)

@dataclass
class Transaction:
    """Represents a transaction in the mempool"""
    hash: HexStr
    from_address: Address
    to_address: Optional[Address]
    value: int
    gas_price: int
    gas_limit: int
    nonce: int
    data: bytes
    timestamp: float

class MempoolMonitor:
    """Monitors mempool for potential MEV opportunities"""
    
    def __init__(self, w3: Web3, config: Dict):
        self.w3 = w3
        self.config = config
        self.transactions: Dict[HexStr, Transaction] = {}
        self.watched_addresses: Set[Address] = set()
        self.running = False
        self._monitor_task = None
        
    async def start(self):
        """Start monitoring the mempool"""
        if self.running:
            return
            
        self.running = True
        self._monitor_task = asyncio.create_task(self._monitor_loop())
        
    async def stop(self):
        """Stop monitoring the mempool"""
        self.running = False
        if self._monitor_task:
            await self._monitor_task
            self._monitor_task = None
        
    def add_watch_address(self, address: Address):
        """Add address to watch list"""
        self.watched_addresses.add(address)
        
    def remove_watch_address(self, address: Address):
        """Remove address from watch list"""
        self.watched_addresses.discard(address)
        
    async def get_transaction(self, tx_hash: HexStr) -> Optional[Transaction]:
        """Get transaction details from mempool"""
        return self.transactions.get(tx_hash)
        
    async def _monitor_loop(self):
        """Main monitoring loop"""
        while self.running:
            try:
                # Get pending transactions
                pending = await self.w3.eth.get_pending_transactions()
                
                # Process each transaction
                for tx in pending:
                    tx_hash = tx['hash'].hex()
                    
                    # Skip if already processed
                    if tx_hash in self.transactions:
                        continue
                        
                    # Create transaction object
                    transaction = Transaction(
                        hash=tx_hash,
                        from_address=tx['from'],
                        to_address=tx.get('to'),
                        value=tx['value'],
                        gas_price=tx['gasPrice'],
                        gas_limit=tx['gas'],
                        nonce=tx['nonce'],
                        data=tx.get('input', b''),
                        timestamp=time.time()
                    )
                    
                    # Store transaction
                    self.transactions[tx_hash] = transaction
                    
                    # Log if it involves watched address
                    if (transaction.from_address in self.watched_addresses or
                        transaction.to_address in self.watched_addresses):
                        logger.info(f"Detected transaction involving watched address: {tx_hash}")
                
                # Clean up old transactions
                current_time = time.time()
                old_txs = [
                    hash for hash, tx in self.transactions.items()
                    if current_time - tx.timestamp > 300  # 5 minutes
                ]
                
                for tx_hash in old_txs:
                    del self.transactions[tx_hash]
                    
            except Exception as e:
                logger.error(f"Error in monitor loop: {e}")
                
            # Add delay between iterations
            await asyncio.sleep(1)

class MempoolManager:
    def __init__(self, w3: Web3, config: Dict):
        self.w3 = w3
        self.config = config
        self.pending_txs: Dict[HexStr, Transaction] = {}
        self.known_bots: Set[Address] = set()
        self.executor = ThreadPoolExecutor(max_workers=4)
        
        # Performance optimizations
        self.enable_custom_indexing = True
        self.index_by_protocol = {}  # Protocol-specific transaction indexing
        self.index_by_token = {}     # Token-specific transaction indexing
        
    async def start_monitoring(self):
        """
        Start monitoring the mempool with optimized indexing
        """
        try:
            # Subscribe to pending transactions
            pending_filter = await self.w3.eth.filter('pending')
            
            while True:
                try:
                    # Get new pending transactions
                    new_pending = await pending_filter.get_new_entries()
                    
                    # Process transactions in parallel
                    await asyncio.gather(*[
                        self._process_transaction(tx_hash)
                        for tx_hash in new_pending
                    ])
                    
                    # Clean up old transactions
                    self._cleanup_old_transactions()
                    
                except Exception as e:
                    logger.error(f"Error processing pending transactions: {e}")
                    await asyncio.sleep(1)
                
        except Exception as e:
            logger.error(f"Error in mempool monitoring: {e}")
            raise
    
    async def _process_transaction(self, tx_hash: HexStr):
        """
        Process and index a new transaction
        """
        try:
            # Get transaction details
            tx = await self.w3.eth.get_transaction(tx_hash)
            
            if not tx:
                return
            
            # Create transaction object
            transaction = Transaction(
                hash=tx_hash,
                from_address=tx['from'],
                to_address=tx.get('to'),
                value=tx.get('value', 0),
                gas_price=tx.get('gasPrice', 0),
                gas_limit=tx.get('gas', 0),
                nonce=tx.get('nonce', 0),
                data=tx.get('input', b''),
                timestamp=time.time()
            )
            
            # Store transaction
            self.pending_txs[tx_hash] = transaction
            
            # Update custom indexes if enabled
            if self.enable_custom_indexing:
                await self._update_indexes(transaction)
            
            # Analyze for MEV opportunities
            if self._is_relevant_transaction(transaction):
                await self._analyze_mev_opportunity(transaction)
            
        except Exception as e:
            logger.error(f"Error processing transaction {tx_hash}: {e}")
    
    async def _update_indexes(self, transaction: Transaction):
        """
        Update custom indexes for faster querying
        """
        try:
            # Decode transaction input
            protocol = self._identify_protocol(transaction.data)
            if protocol:
                self.index_by_protocol.setdefault(protocol, set()).add(transaction.hash)
            
            # Index by token
            token = self._identify_token(transaction.to_address)
            if token:
                self.index_by_token.setdefault(token, set()).add(transaction.hash)
            
        except Exception as e:
            logger.error(f"Error updating indexes: {e}")
    
    def _identify_protocol(self, data: bytes) -> Optional[str]:
        """
        Identify the protocol based on transaction input data
        """
        # Implement protocol identification logic
        # This is a placeholder
        return None
    
    def _identify_token(self, address: Optional[Address]) -> Optional[str]:
        """
        Identify token from address
        """
        # Implement token identification logic
        # This is a placeholder
        return None
    
    def _is_relevant_transaction(self, transaction: Transaction) -> bool:
        """
        Check if transaction is relevant for MEV opportunities
        """
        # Implement relevance checking logic
        # This is a placeholder
        return False
    
    async def _analyze_mev_opportunity(self, transaction: Transaction):
        """
        Analyze transaction for MEV opportunities
        """
        # Implement MEV opportunity analysis
        # This is a placeholder
        pass
    
    def _cleanup_old_transactions(self):
        """
        Clean up old transactions from memory
        """
        current_time = time.time()
        old_threshold = current_time - self.config.get('tx_cleanup_threshold', 60)
        
        # Remove old transactions
        old_txs = [
            tx_hash for tx_hash, tx in self.pending_txs.items()
            if tx.timestamp < old_threshold
        ]
        
        for tx_hash in old_txs:
            self._remove_transaction(tx_hash)
    
    def _remove_transaction(self, tx_hash: HexStr):
        """
        Remove transaction and update indexes
        """
        if tx_hash in self.pending_txs:
            tx = self.pending_txs[tx_hash]
            
            # Remove from main storage
            del self.pending_txs[tx_hash]
            
            # Remove from indexes
            if self.enable_custom_indexing:
                self._remove_from_indexes(tx_hash)

    async def scan_mempool(self) -> List[Dict]:
        """
        Scan mempool for potential transactions
        """
        try:
            # Get pending transactions
            pending_block = await self.w3.eth.get_block('pending', full_transactions=True)
            return pending_block.transactions
            
        except Exception as e:
            logger.error(f"Error scanning mempool: {e}")
            return []
            
    async def filter_transactions(self, 
                                transactions: List[Dict],
                                min_value: Optional[int] = None,
                                max_gas: Optional[int] = None) -> List[Dict]:
        """
        Filter transactions based on criteria
        """
        filtered = []
        
        for tx in transactions:
            # Apply value filter
            if min_value is not None and int(tx['value']) < min_value:
                continue
                
            # Apply gas filter
            if max_gas is not None and int(tx['gas']) > max_gas:
                continue
                
            filtered.append(tx)
            
        return filtered
