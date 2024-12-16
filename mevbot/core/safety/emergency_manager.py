"""Emergency management system for MEV bot."""
import os
import time
import json
import asyncio
import logging
import smtplib
import requests
from typing import Dict, List, Optional, Set
from enum import Enum
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart
from dataclasses import dataclass
from datetime import datetime
from web3 import Web3
from eth_account import Account

logger = logging.getLogger(__name__)

class EmergencyLevel(Enum):
    """Emergency severity levels."""
    INFO = "INFO"
    WARNING = "WARNING"
    CRITICAL = "CRITICAL"
    FATAL = "FATAL"

@dataclass
class EmergencyEvent:
    """Represents an emergency event."""
    level: EmergencyLevel
    message: str
    timestamp: float
    source: str
    tx_hash: Optional[str] = None
    additional_data: Optional[Dict] = None

class EmergencyManager:
    """
    Manages emergency situations and responses for the MEV bot.
    
    Features:
    - Emergency shutdown procedures
    - Position unwinding
    - Alert notifications (email, Discord, Telegram)
    - Transaction cancellation
    - System state preservation
    - Automatic recovery procedures
    """
    
    def __init__(self, config: Dict, web3: Web3):
        """Initialize emergency manager with configuration."""
        self.config = config
        self.web3 = web3
        self.shutdown_triggered = False
        self.recovery_mode = False
        
        # Initialize notification settings
        self.notification_config = config.get('notifications', {})
        self.emergency_contacts = set(config.get('emergency_contacts', []))
        self.discord_webhook = config.get('discord_webhook')
        self.telegram_token = config.get('telegram_token')
        self.telegram_chat_ids = set(config.get('telegram_chat_ids', []))
        
        # Email configuration
        self.email_config = config.get('email', {})
        self.smtp_server = self.email_config.get('smtp_server')
        self.smtp_port = self.email_config.get('smtp_port')
        self.smtp_username = self.email_config.get('username')
        self.smtp_password = self.email_config.get('password')
        
        # Transaction tracking
        self.pending_transactions: Set[str] = set()
        self.cancelled_transactions: Set[str] = set()
        
        # State preservation
        self.state_file = os.path.expanduser('~/.mevbot/emergency_state.json')
        self._load_state()
        
        # Background tasks
        self._background_tasks = set()
        self._start_background_tasks()
    
    def _start_background_tasks(self) -> None:
        """Start background monitoring tasks."""
        self._background_tasks.add(
            asyncio.create_task(self._monitor_pending_transactions())
        )
        self._background_tasks.add(
            asyncio.create_task(self._periodic_state_backup())
        )
    
    async def trigger_shutdown(self, reason: str, level: EmergencyLevel) -> None:
        """
        Trigger emergency shutdown procedure.
        
        Args:
            reason: Reason for shutdown
            level: Emergency severity level
        """
        if self.shutdown_triggered:
            logger.warning("Emergency shutdown already triggered")
            return
        
        self.shutdown_triggered = True
        event = EmergencyEvent(
            level=level,
            message=f"Emergency shutdown triggered: {reason}",
            timestamp=time.time(),
            source="EmergencyManager"
        )
        
        logger.critical(f"EMERGENCY SHUTDOWN: {reason}")
        
        try:
            # Execute shutdown tasks in parallel
            await asyncio.gather(
                self._notify_emergency_contacts(event),
                self._unwind_positions(),
                self._cancel_pending_transactions(),
                self._preserve_system_state(),
                return_exceptions=True
            )
        except Exception as e:
            logger.error(f"Error during emergency shutdown: {str(e)}")
            # Even if some tasks fail, continue with shutdown
        
        # Force exit if FATAL level
        if level == EmergencyLevel.FATAL:
            logger.critical("FATAL emergency level - forcing exit")
            os._exit(1)
    
    async def _unwind_positions(self) -> None:
        """Unwind all open positions."""
        try:
            # Implementation will depend on specific trading strategies
            # This is a placeholder for the actual implementation
            logger.info("Unwinding positions...")
            # Add position unwinding logic here
            pass
        except Exception as e:
            logger.error(f"Error unwinding positions: {str(e)}")
    
    async def _cancel_pending_transactions(self) -> None:
        """Cancel all pending transactions."""
        try:
            for tx_hash in self.pending_transactions.copy():
                try:
                    # Send cancellation transaction with higher gas price
                    tx = await self._get_transaction(tx_hash)
                    if tx and tx['from']:
                        cancel_tx = {
                            'from': tx['from'],
                            'to': tx['from'],
                            'value': 0,
                            'nonce': tx['nonce'],
                            'gasPrice': int(tx['gasPrice'] * 1.5),  # 50% higher gas price
                            'gas': 21000
                        }
                        
                        # Sign and send cancellation transaction
                        signed_tx = Account.sign_transaction(cancel_tx, 
                            self.config['private_key'])
                        await self.web3.eth.send_raw_transaction(signed_tx.rawTransaction)
                        
                        self.cancelled_transactions.add(tx_hash)
                        self.pending_transactions.remove(tx_hash)
                        logger.info(f"Cancelled transaction {tx_hash}")
                except Exception as e:
                    logger.error(f"Error cancelling transaction {tx_hash}: {str(e)}")
        except Exception as e:
            logger.error(f"Error in transaction cancellation: {str(e)}")
    
    async def _preserve_system_state(self) -> None:
        """Preserve current system state."""
        try:
            state = {
                'timestamp': time.time(),
                'shutdown_triggered': self.shutdown_triggered,
                'pending_transactions': list(self.pending_transactions),
                'cancelled_transactions': list(self.cancelled_transactions)
            }
            
            os.makedirs(os.path.dirname(self.state_file), exist_ok=True)
            with open(self.state_file, 'w') as f:
                json.dump(state, f)
            logger.info("System state preserved")
        except Exception as e:
            logger.error(f"Error preserving system state: {str(e)}")
    
    def _load_state(self) -> None:
        """Load preserved system state."""
        try:
            if os.path.exists(self.state_file):
                with open(self.state_file, 'r') as f:
                    state = json.load(f)
                self.pending_transactions = set(state.get('pending_transactions', []))
                self.cancelled_transactions = set(state.get('cancelled_transactions', []))
                if state.get('shutdown_triggered'):
                    self.recovery_mode = True
        except Exception as e:
            logger.error(f"Error loading system state: {str(e)}")
    
    async def _notify_emergency_contacts(self, event: EmergencyEvent) -> None:
        """Notify all emergency contacts through configured channels."""
        notification_tasks = []
        
        # Email notifications
        if self.smtp_server and self.emergency_contacts:
            notification_tasks.append(self._send_email_alert(event))
        
        # Discord notifications
        if self.discord_webhook:
            notification_tasks.append(self._send_discord_alert(event))
        
        # Telegram notifications
        if self.telegram_token and self.telegram_chat_ids:
            notification_tasks.append(self._send_telegram_alert(event))
        
        # Execute all notifications in parallel
        await asyncio.gather(*notification_tasks, return_exceptions=True)
    
    async def _send_email_alert(self, event: EmergencyEvent) -> None:
        """Send email alert to emergency contacts."""
        try:
            msg = MIMEMultipart()
            msg['Subject'] = f"MEV Bot Emergency: {event.level.value}"
            msg['From'] = self.smtp_username
            
            body = f"""
            Emergency Event Details:
            Level: {event.level.value}
            Time: {datetime.fromtimestamp(event.timestamp)}
            Source: {event.source}
            Message: {event.message}
            """
            if event.tx_hash:
                body += f"\nTransaction: {event.tx_hash}"
            
            msg.attach(MIMEText(body, 'plain'))
            
            with smtplib.SMTP(self.smtp_server, self.smtp_port) as server:
                server.starttls()
                server.login(self.smtp_username, self.smtp_password)
                for contact in self.emergency_contacts:
                    msg['To'] = contact
                    server.send_message(msg)
            
            logger.info("Email alerts sent")
        except Exception as e:
            logger.error(f"Error sending email alert: {str(e)}")
    
    async def _send_discord_alert(self, event: EmergencyEvent) -> None:
        """Send Discord alert."""
        try:
            message = {
                'content': f"**MEV Bot Emergency Alert**\n"
                          f"Level: {event.level.value}\n"
                          f"Time: {datetime.fromtimestamp(event.timestamp)}\n"
                          f"Source: {event.source}\n"
                          f"Message: {event.message}"
            }
            if event.tx_hash:
                message['content'] += f"\nTransaction: {event.tx_hash}"
            
            async with aiohttp.ClientSession() as session:
                async with session.post(self.discord_webhook, json=message) as response:
                    if response.status != 204:
                        logger.error(f"Discord notification failed: {await response.text()}")
        except Exception as e:
            logger.error(f"Error sending Discord alert: {str(e)}")
    
    async def _send_telegram_alert(self, event: EmergencyEvent) -> None:
        """Send Telegram alert."""
        try:
            message = (f"🚨 *MEV Bot Emergency Alert*\n"
                      f"Level: {event.level.value}\n"
                      f"Time: {datetime.fromtimestamp(event.timestamp)}\n"
                      f"Source: {event.source}\n"
                      f"Message: {event.message}")
            if event.tx_hash:
                message += f"\nTransaction: {event.tx_hash}"
            
            url = f"https://api.telegram.org/bot{self.telegram_token}/sendMessage"
            for chat_id in self.telegram_chat_ids:
                params = {
                    'chat_id': chat_id,
                    'text': message,
                    'parse_mode': 'Markdown'
                }
                async with aiohttp.ClientSession() as session:
                    async with session.get(url, params=params) as response:
                        if response.status != 200:
                            logger.error(f"Telegram notification failed: {await response.text()}")
        except Exception as e:
            logger.error(f"Error sending Telegram alert: {str(e)}")
    
    async def _monitor_pending_transactions(self) -> None:
        """Monitor and manage pending transactions."""
        while True:
            try:
                for tx_hash in self.pending_transactions.copy():
                    receipt = await self.web3.eth.get_transaction_receipt(tx_hash)
                    if receipt:
                        self.pending_transactions.remove(tx_hash)
                        if receipt['status'] == 0:
                            logger.warning(f"Transaction failed: {tx_hash}")
                            # Handle failed transaction
                await asyncio.sleep(1)
            except Exception as e:
                logger.error(f"Error monitoring transactions: {str(e)}")
                await asyncio.sleep(5)
    
    async def _periodic_state_backup(self) -> None:
        """Periodically backup system state."""
        while True:
            try:
                await self._preserve_system_state()
                await asyncio.sleep(300)  # Every 5 minutes
            except Exception as e:
                logger.error(f"Error in periodic state backup: {str(e)}")
                await asyncio.sleep(60)
    
    async def _get_transaction(self, tx_hash: str) -> Optional[Dict]:
        """Get transaction details by hash."""
        try:
            return await self.web3.eth.get_transaction(tx_hash)
        except Exception as e:
            logger.error(f"Error getting transaction {tx_hash}: {str(e)}")
            return None
    
    def add_emergency_contact(self, contact: str) -> None:
        """Add new emergency contact."""
        self.emergency_contacts.add(contact)
    
    def remove_emergency_contact(self, contact: str) -> None:
        """Remove emergency contact."""
        self.emergency_contacts.discard(contact)
    
    async def cleanup(self) -> None:
        """Cleanup resources before shutdown."""
        for task in self._background_tasks:
            task.cancel()
        await asyncio.gather(*self._background_tasks, return_exceptions=True)
        self._background_tasks.clear()
