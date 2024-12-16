"""Safety system coordinator for MEV bot."""
import os
import time
import asyncio
import logging
from typing import Dict, Optional, Set
from dataclasses import dataclass
from web3 import Web3

from mevbot.core.security.key_manager import SecureKeyManager
from mevbot.core.safety.circuit_breaker import CircuitBreaker
from mevbot.core.safety.emergency_manager import EmergencyManager, EmergencyLevel, EmergencyEvent

logger = logging.getLogger(__name__)

@dataclass
class SafetyMetrics:
    """Safety system metrics."""
    total_transactions: int = 0
    failed_transactions: int = 0
    emergency_events: int = 0
    circuit_breaks: int = 0
    last_incident_time: Optional[float] = None
    total_prevented_loss: float = 0.0
    current_risk_level: str = "LOW"

class SafetyCoordinator:
    """
    Coordinates all safety systems for the MEV bot.
    
    Features:
    - Unified safety configuration
    - Coordinated emergency response
    - Centralized logging and monitoring
    - Integrated safety checks
    - System-wide safety metrics
    """
    
    def __init__(self, config: Dict, web3: Web3):
        """Initialize safety coordinator with configuration."""
        self.config = config
        self.web3 = web3
        self.metrics = SafetyMetrics()
        
        # Initialize safety components
        self.key_manager = SecureKeyManager(config.get('key_manager', {}))
        self.circuit_breaker = CircuitBreaker(config.get('circuit_breaker', {}))
        self.emergency_manager = EmergencyManager(config.get('emergency_manager', {}), web3)
        
        # Track active strategies
        self.active_strategies: Set[str] = set()
        
        # Background monitoring
        self._monitoring_task = None
        self._metrics_task = None
        
        # Initialize logging
        self._setup_logging()
    
    def _setup_logging(self) -> None:
        """Setup unified logging for all safety components."""
        log_dir = os.path.expanduser('~/.mevbot/logs')
        os.makedirs(log_dir, exist_ok=True)
        
        # File handler for all safety events
        safety_handler = logging.FileHandler(
            os.path.join(log_dir, 'safety.log')
        )
        safety_handler.setLevel(logging.INFO)
        formatter = logging.Formatter(
            '%(asctime)s - %(name)s - %(levelname)s - %(message)s'
        )
        safety_handler.setFormatter(formatter)
        
        # Add handler to all safety components
        logging.getLogger('mevbot.core.safety').addHandler(safety_handler)
    
    async def start(self) -> None:
        """Start safety coordination."""
        logger.info("Starting safety coordination system")
        
        # Start background tasks
        self._monitoring_task = asyncio.create_task(self._monitor_safety())
        self._metrics_task = asyncio.create_task(self._update_metrics())
        
        # Initialize emergency manager
        await self._initialize_emergency_system()
    
    async def stop(self) -> None:
        """Stop safety coordination."""
        logger.info("Stopping safety coordination system")
        
        # Cancel background tasks
        if self._monitoring_task:
            self._monitoring_task.cancel()
        if self._metrics_task:
            self._metrics_task.cancel()
        
        # Cleanup components
        await self.emergency_manager.cleanup()
    
    async def validate_transaction(self, tx: Dict) -> bool:
        """
        Validate transaction against all safety checks.
        
        Args:
            tx: Transaction dictionary with details
        
        Returns:
            bool: True if transaction is safe, False otherwise
        """
        try:
            # Check circuit breaker first
            if not await self.circuit_breaker.validate_transaction(tx):
                self.metrics.circuit_breaks += 1
                return False
            
            # Additional safety checks
            if not self._validate_gas_price(tx):
                return False
            
            if not self._validate_contract_interaction(tx):
                return False
            
            self.metrics.total_transactions += 1
            return True
            
        except Exception as e:
            logger.error(f"Error validating transaction: {str(e)}")
            return False
    
    def _validate_gas_price(self, tx: Dict) -> bool:
        """Validate gas price is within safe limits."""
        try:
            gas_price = self.web3.from_wei(tx['gasPrice'], 'gwei')
            max_gas = self.config.get('max_gas_price_gwei', 1000)
            
            if gas_price > max_gas:
                logger.warning(f"Gas price {gas_price} gwei exceeds maximum {max_gas}")
                return False
            
            return True
        except Exception as e:
            logger.error(f"Error validating gas price: {str(e)}")
            return False
    
    def _validate_contract_interaction(self, tx: Dict) -> bool:
        """Validate contract interaction is safe."""
        try:
            # Check if contract is whitelisted
            if 'to' in tx and tx['to']:
                contract_address = tx['to']
                whitelist = self.config.get('contract_whitelist', [])
                
                if contract_address not in whitelist:
                    logger.warning(f"Contract {contract_address} not in whitelist")
                    return False
            
            return True
        except Exception as e:
            logger.error(f"Error validating contract interaction: {str(e)}")
            return False
    
    async def register_strategy(self, strategy_id: str) -> None:
        """Register an active trading strategy."""
        self.active_strategies.add(strategy_id)
        logger.info(f"Registered strategy: {strategy_id}")
    
    async def unregister_strategy(self, strategy_id: str) -> None:
        """Unregister a trading strategy."""
        self.active_strategies.discard(strategy_id)
        logger.info(f"Unregistered strategy: {strategy_id}")
    
    async def record_incident(self, 
                            level: EmergencyLevel,
                            message: str,
                            source: str,
                            tx_hash: Optional[str] = None) -> None:
        """
        Record a safety incident.
        
        Args:
            level: Emergency severity level
            message: Incident description
            source: Source of the incident
            tx_hash: Related transaction hash if any
        """
        self.metrics.emergency_events += 1
        self.metrics.last_incident_time = time.time()
        
        event = EmergencyEvent(
            level=level,
            message=message,
            timestamp=time.time(),
            source=source,
            tx_hash=tx_hash
        )
        
        # Update risk level
        self._update_risk_level(level)
        
        # Notify emergency manager
        if level in [EmergencyLevel.CRITICAL, EmergencyLevel.FATAL]:
            await self.emergency_manager.trigger_shutdown(message, level)
    
    def _update_risk_level(self, incident_level: EmergencyLevel) -> None:
        """Update current risk level based on incident."""
        level_weights = {
            EmergencyLevel.INFO: 1,
            EmergencyLevel.WARNING: 2,
            EmergencyLevel.CRITICAL: 3,
            EmergencyLevel.FATAL: 4
        }
        
        weight = level_weights[incident_level]
        
        if weight >= 3:
            self.metrics.current_risk_level = "HIGH"
        elif weight == 2:
            self.metrics.current_risk_level = "MEDIUM"
        else:
            self.metrics.current_risk_level = "LOW"
    
    async def _monitor_safety(self) -> None:
        """Monitor overall system safety."""
        while True:
            try:
                # Check system resources
                if not self._check_system_resources():
                    await self.record_incident(
                        EmergencyLevel.WARNING,
                        "System resources critical",
                        "SafetyCoordinator"
                    )
                
                # Check network conditions
                if not await self._check_network_conditions():
                    await self.record_incident(
                        EmergencyLevel.WARNING,
                        "Network conditions unfavorable",
                        "SafetyCoordinator"
                    )
                
                await asyncio.sleep(60)  # Check every minute
                
            except Exception as e:
                logger.error(f"Error in safety monitoring: {str(e)}")
                await asyncio.sleep(5)
    
    async def _update_metrics(self) -> None:
        """Update safety metrics periodically."""
        while True:
            try:
                # Calculate prevented loss
                self.metrics.total_prevented_loss = (
                    self.metrics.circuit_breaks * 
                    float(self.config.get('average_transaction_value', 0.1))
                )
                
                # Log metrics
                logger.info(f"Safety Metrics: {self.metrics}")
                
                await asyncio.sleep(300)  # Update every 5 minutes
                
            except Exception as e:
                logger.error(f"Error updating metrics: {str(e)}")
                await asyncio.sleep(60)
    
    def _check_system_resources(self) -> bool:
        """Check if system resources are within safe limits."""
        try:
            import psutil
            
            # Check CPU usage
            cpu_percent = psutil.cpu_percent()
            if cpu_percent > 90:
                logger.warning(f"High CPU usage: {cpu_percent}%")
                return False
            
            # Check memory usage
            memory = psutil.virtual_memory()
            if memory.percent > 90:
                logger.warning(f"High memory usage: {memory.percent}%")
                return False
            
            # Check disk usage
            disk = psutil.disk_usage('/')
            if disk.percent > 90:
                logger.warning(f"High disk usage: {disk.percent}%")
                return False
            
            return True
            
        except Exception as e:
            logger.error(f"Error checking system resources: {str(e)}")
            return False
    
    async def _check_network_conditions(self) -> bool:
        """Check if network conditions are favorable."""
        try:
            # Check gas price
            gas_price = self.web3.eth.gas_price
            max_gas = self.web3.to_wei(
                self.config.get('max_gas_price_gwei', 1000),
                'gwei'
            )
            
            if gas_price > max_gas:
                logger.warning(f"High gas price: {self.web3.from_wei(gas_price, 'gwei')} gwei")
                return False
            
            # Check block time
            latest_block = await self.web3.eth.get_block('latest')
            if time.time() - latest_block['timestamp'] > 60:
                logger.warning("Network might be congested")
                return False
            
            return True
            
        except Exception as e:
            logger.error(f"Error checking network conditions: {str(e)}")
            return False
    
    async def _initialize_emergency_system(self) -> None:
        """Initialize emergency response system."""
        try:
            # Connect circuit breaker to emergency manager
            self.circuit_breaker.on_trigger = self.emergency_manager.trigger_shutdown
            
            # Initialize recovery if needed
            if self.emergency_manager.recovery_mode:
                logger.warning("Starting in recovery mode")
                await self._handle_recovery()
            
        except Exception as e:
            logger.error(f"Error initializing emergency system: {str(e)}")
            raise
    
    async def _handle_recovery(self) -> None:
        """Handle recovery from previous emergency shutdown."""
        try:
            # Check preserved state
            # Implement recovery logic here
            pass
        except Exception as e:
            logger.error(f"Error in recovery: {str(e)}")
            raise
