from setuptools import setup, find_packages

setup(
    name="mevbot",
    version="0.1.0",
    packages=find_packages(),
    install_requires=[
        "web3>=6.0.0",
        "numpy>=1.21.0",
        "pytest>=7.0.0",
        "pytest-asyncio>=0.20.0",
        "python-dotenv>=0.19.0"
    ],
)
