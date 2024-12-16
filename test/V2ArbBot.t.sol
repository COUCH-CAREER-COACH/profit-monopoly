// SPDX-License-Identifier: MIT
pragma solidity ^0.8.13;

import "forge-std/Test.sol";
import "../src/V2ArbBot.sol";
import "./mocks/MockToken.sol";
import "./mocks/MockV2Pool.sol";

contract V2ArbBotTest is Test {
    V2ArbBot public bot;
    address public owner;
    MockToken public weth;
    MockToken public tokenA;
    MockToken public tokenB;
    MockV2Pool public poolAB;
    
    function setUp() public {
        owner = address(this);
        weth = new MockToken("Wrapped ETH", "WETH", 18);
        tokenA = new MockToken("Token A", "TKA", 18);
        tokenB = new MockToken("Token B", "TKB", 18);
        
        bot = new V2ArbBot(owner, address(weth));
        poolAB = new MockV2Pool(address(tokenA), address(tokenB));
        
        // Setup initial liquidity
        tokenA.mint(address(poolAB), 1000e18);
        tokenB.mint(address(poolAB), 1000e18);
    }
    
    function testEmergencyShutdown() public {
        // Test emergency shutdown functionality
        assertTrue(!bot.isShutdown(), "Bot should not be shutdown initially");
        bot.emergencyShutdown();
        assertTrue(bot.isShutdown(), "Bot should be shutdown");
    }
    
    function testRateLimiting() public {
        // Test rate limiting functionality
        uint256 initialLimit = bot.callsPerBlock();
        assertTrue(initialLimit > 0, "Should have non-zero rate limit");
        
        // Test call counting
        bot.executeArbitrage(address(tokenA), address(tokenB), 1e18, new bytes(0));
        assertEq(bot.callsThisBlock(), 1, "Should count calls correctly");
    }
    
    function testInputValidation() public {
        // Test input validation
        vm.expectRevert("Invalid token address");
        bot.executeArbitrage(address(0), address(tokenB), 1e18, new bytes(0));
        
        vm.expectRevert("Amount must be greater than 0");
        bot.executeArbitrage(address(tokenA), address(tokenB), 0, new bytes(0));
    }
}
