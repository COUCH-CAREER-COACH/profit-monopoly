"""Secure key management system for MEV bot."""
import os
import base64
import json
import logging
from typing import Dict, Optional
from pathlib import Path
from cryptography.fernet import Fernet
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.kdf.pbkdf2 import PBKDF2HMAC
from eth_account import Account
from web3.auto import w3

logger = logging.getLogger(__name__)

class SecureKeyManager:
    """Manages secure storage and retrieval of private keys."""
    
    def __init__(self, config: Dict):
        """Initialize the key manager with configuration."""
        self.config = config
        self.keystore_path = Path(os.path.expanduser(config.get('keystore_path', '~/.mevbot/keys')))
        self.master_key_path = self.keystore_path / '.master.key'
        self.keys_file = self.keystore_path / 'encrypted_keys.json'
        self.fernet = None
        self._initialize_keystore()
    
    def _initialize_keystore(self) -> None:
        """Create and secure the keystore directory."""
        try:
            # Create keystore directory with secure permissions
            self.keystore_path.mkdir(parents=True, mode=0o700, exist_ok=True)
            
            # Generate master key if it doesn't exist
            if not self.master_key_path.exists():
                master_key = Fernet.generate_key()
                with open(self.master_key_path, 'wb') as f:
                    f.write(master_key)
                os.chmod(self.master_key_path, 0o600)
                
            # Initialize encryption
            with open(self.master_key_path, 'rb') as f:
                master_key = f.read()
            self.fernet = Fernet(master_key)
            
            # Initialize keys file if it doesn't exist
            if not self.keys_file.exists():
                with open(self.keys_file, 'w') as f:
                    json.dump({}, f)
                os.chmod(self.keys_file, 0o600)
                
        except Exception as e:
            logger.error(f"Failed to initialize keystore: {str(e)}")
            raise
    
    def add_private_key(self, key_id: str, private_key: str, password: str) -> bool:
        """
        Add a new private key to the secure storage.
        
        Args:
            key_id: Identifier for the key
            private_key: The private key to store
            password: Password to encrypt the key
        
        Returns:
            bool: True if successful, False otherwise
        """
        try:
            # Validate private key format
            if not self._validate_private_key(private_key):
                logger.error(f"Invalid private key format for key_id: {key_id}")
                return False
            
            # Generate salt and derive key
            salt = os.urandom(16)
            kdf = PBKDF2HMAC(
                algorithm=hashes.SHA256(),
                length=32,
                salt=salt,
                iterations=100000,
            )
            derived_key = base64.urlsafe_b64encode(kdf.derive(password.encode()))
            
            # Create a new Fernet instance with the derived key
            f = Fernet(derived_key)
            
            # Encrypt private key with derived key
            encrypted_key = f.encrypt(private_key.encode())
            
            # Load existing keys
            with open(self.keys_file, 'r') as f:
                keys = json.load(f)
            
            # Store new key with salt
            keys[key_id] = {
                'encrypted_key': encrypted_key.decode(),
                'salt': base64.b64encode(salt).decode(),
                'address': Account.from_key(private_key).address
            }
            
            # Save updated keys
            with open(self.keys_file, 'w') as f:
                json.dump(keys, f)
            
            logger.info(f"Successfully added key with ID: {key_id}")
            return True
            
        except Exception as e:
            logger.error(f"Failed to add private key: {str(e)}")
            return False
    
    def get_private_key(self, key_id: str, password: str) -> Optional[str]:
        """
        Retrieve a private key from secure storage.
        
        Args:
            key_id: Identifier for the key
            password: Password to decrypt the key
        
        Returns:
            Optional[str]: The private key if successful, None otherwise
        """
        try:
            # Load keys
            with open(self.keys_file, 'r') as f:
                keys = json.load(f)
            
            if key_id not in keys:
                logger.error(f"Key ID not found: {key_id}")
                return None
            
            key_data = keys[key_id]
            salt = base64.b64decode(key_data['salt'])
            
            # Derive key using same parameters
            kdf = PBKDF2HMAC(
                algorithm=hashes.SHA256(),
                length=32,
                salt=salt,
                iterations=100000,
            )
            derived_key = base64.urlsafe_b64encode(kdf.derive(password.encode()))
            
            # Create a new Fernet instance with the derived key
            f = Fernet(derived_key)
            
            try:
                # Decrypt private key
                encrypted_key = key_data['encrypted_key'].encode()
                private_key = f.decrypt(encrypted_key).decode()
                
                # Validate decrypted key
                if not self._validate_private_key(private_key):
                    logger.error(f"Decrypted key validation failed for key_id: {key_id}")
                    return None
                    
                return private_key
                
            except Exception:
                logger.error("Failed to decrypt key - invalid password")
                return None
                
        except Exception as e:
            logger.error(f"Failed to retrieve private key: {str(e)}")
            return None
    
    def list_keys(self) -> Dict[str, str]:
        """
        List all stored keys and their associated addresses.
        
        Returns:
            Dict[str, str]: Mapping of key IDs to their Ethereum addresses
        """
        try:
            with open(self.keys_file, 'r') as f:
                keys = json.load(f)
            return {k: v['address'] for k, v in keys.items()}
        except Exception as e:
            logger.error(f"Failed to list keys: {str(e)}")
            return {}
    
    def remove_key(self, key_id: str, password: str) -> bool:
        """
        Remove a key from secure storage.
        
        Args:
            key_id: Identifier for the key to remove
            password: Password to verify authority
        
        Returns:
            bool: True if successful, False otherwise
        """
        try:
            # Verify password by attempting to decrypt the key
            if self.get_private_key(key_id, password) is None:
                logger.error(f"Failed to verify password for key removal: {key_id}")
                return False
            
            # Load keys
            with open(self.keys_file, 'r') as f:
                keys = json.load(f)
            
            # Remove key
            if key_id in keys:
                del keys[key_id]
                
                # Save updated keys
                with open(self.keys_file, 'w') as f:
                    json.dump(keys, f)
                
                logger.info(f"Successfully removed key: {key_id}")
                return True
            else:
                logger.error(f"Key not found for removal: {key_id}")
                return False
                
        except Exception as e:
            logger.error(f"Failed to remove key: {str(e)}")
            return False
    
    def _validate_private_key(self, private_key: str) -> bool:
        """
        Validate private key format and derivable address.
        
        Args:
            private_key: The private key to validate
        
        Returns:
            bool: True if valid, False otherwise
        """
        try:
            # Remove '0x' prefix if present
            if private_key.startswith('0x'):
                private_key = private_key[2:]
            
            # Check length
            if len(private_key) != 64:
                return False
            
            # Check hex format
            int(private_key, 16)
            
            # Try to derive address
            account = Account.from_key(private_key)
            return w3.is_address(account.address)
            
        except Exception:
            return False
