"""Test runner for MEV bot strategies"""
import pytest
import asyncio
import sys
import os
from typing import List
import logging

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

def run_tests(test_paths: List[str] = None):
    """Run pytest with specified test paths"""
    if test_paths is None:
        test_paths = ['tests']
        
    try:
        # Add project root to Python path
        project_root = os.path.dirname(os.path.abspath(__file__))
        sys.path.insert(0, project_root)
        
        # Run tests with pytest
        args = [
            '-v',  # Verbose output
            '--asyncio-mode=auto',  # Handle async tests
            '-s',  # Show print statements
            '--tb=short',  # Shorter traceback
            '--color=yes'  # Colored output
        ]
        args.extend(test_paths)
        
        exit_code = pytest.main(args)
        
        if exit_code == 0:
            logger.info("All tests passed successfully! 🚀")
        else:
            logger.error("Some tests failed. Please check the output above.")
            
        return exit_code
        
    except Exception as e:
        logger.error(f"Error running tests: {e}")
        return 1

if __name__ == "__main__":
    # Get test paths from command line args
    test_paths = sys.argv[1:] if len(sys.argv) > 1 else None
    
    # Run tests
    exit_code = run_tests(test_paths)
    sys.exit(exit_code)
