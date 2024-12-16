import pytest
import os
import ctypes
from dotenv import load_dotenv

# Load environment variables for all tests
@pytest.fixture(autouse=True)
def load_env():
    load_dotenv()
    
    # Verify required environment variables
    required_vars = [
        'HTTPS_URL_SEPOLIA',
        'WSS_URL_SEPOLIA',
        'PRIVATE_KEY'
    ]
    
    missing_vars = [var for var in required_vars if not os.getenv(var)]
    if missing_vars:
        pytest.skip(f"Missing required environment variables: {', '.join(missing_vars)}")

def pytest_configure():
    """Configure pytest with custom markers and variables"""
    # Check if DPDK is available
    try:
        dpdk = ctypes.CDLL('librte.so')
        pytest.DPDK_AVAILABLE = True
    except OSError:
        pytest.DPDK_AVAILABLE = False

# Add any shared fixtures here
