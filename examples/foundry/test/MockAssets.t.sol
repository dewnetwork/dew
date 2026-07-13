// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {Test} from "forge-std/Test.sol";
import {IERC20Errors} from "@openzeppelin/contracts/interfaces/draft-IERC6093.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";
import {MockERC20} from "../src/MockERC20.sol";
import {MockUSDT} from "../src/MockUSDT.sol";
import {MockUSDC} from "../src/MockUSDC.sol";
import {MockDAI} from "../src/MockDAI.sol";
import {MockWETH} from "../src/MockWETH.sol";
import {MockWBTC} from "../src/MockWBTC.sol";

contract MockAssetsTest is Test {
    address internal alice = address(0xA11CE);
    address internal bob = address(0xB0B);

    function test_mockERC20_customMetadata() public {
        vm.prank(alice);
        MockERC20 t = new MockERC20("Custom", "CST", 9, 1_000e9);
        assertEq(t.name(), "Custom");
        assertEq(t.symbol(), "CST");
        assertEq(t.decimals(), 9);
        assertEq(t.totalSupply(), 1_000e9);
        assertEq(t.balanceOf(alice), 1_000e9);
        assertEq(t.owner(), alice);
    }

    function test_mockERC20_zeroSupplyOk() public {
        vm.prank(alice);
        MockERC20 t = new MockERC20("Empty", "EMP", 18, 0);
        assertEq(t.totalSupply(), 0);
        vm.prank(alice);
        t.mint(bob, 1e18);
        assertEq(t.balanceOf(bob), 1e18);
    }

    function test_usdt_metadata() public {
        vm.prank(alice);
        MockUSDT usdt = new MockUSDT(1_000_000e6);
        assertEq(usdt.name(), "Tether USD");
        assertEq(usdt.symbol(), "USDT");
        assertEq(usdt.decimals(), 6);
        assertEq(usdt.totalSupply(), 1_000_000e6);
    }

    function test_usdc_metadata() public {
        vm.prank(alice);
        MockUSDC usdc = new MockUSDC(1_000_000e6);
        assertEq(usdc.name(), "USD Coin");
        assertEq(usdc.symbol(), "USDC");
        assertEq(usdc.decimals(), 6);
        assertEq(usdc.totalSupply(), 1_000_000e6);
    }

    function test_dai_metadata() public {
        vm.prank(alice);
        MockDAI dai = new MockDAI(1_000_000e18);
        assertEq(dai.name(), "Dai Stablecoin");
        assertEq(dai.symbol(), "DAI");
        assertEq(dai.decimals(), 18);
        assertEq(dai.totalSupply(), 1_000_000e18);
    }

    function test_weth_metadata() public {
        vm.prank(alice);
        MockWETH weth = new MockWETH(10_000e18);
        assertEq(weth.name(), "Wrapped Ether");
        assertEq(weth.symbol(), "WETH");
        assertEq(weth.decimals(), 18);
        assertEq(weth.totalSupply(), 10_000e18);
    }

    function test_wbtc_metadata() public {
        vm.prank(alice);
        MockWBTC wbtc = new MockWBTC(100e8);
        assertEq(wbtc.name(), "Wrapped BTC");
        assertEq(wbtc.symbol(), "WBTC");
        assertEq(wbtc.decimals(), 8);
        assertEq(wbtc.totalSupply(), 100e8);
    }

    function test_transferAndMintAcrossBasket() public {
        vm.startPrank(alice);
        MockUSDT usdt = new MockUSDT(1_000_000e6);
        MockUSDC usdc = new MockUSDC(1_000_000e6);
        MockDAI dai = new MockDAI(1_000_000e18);
        MockWETH weth = new MockWETH(10_000e18);
        MockWBTC wbtc = new MockWBTC(100e8);

        assertTrue(usdt.transfer(bob, 100e6));
        assertTrue(usdc.transfer(bob, 100e6));
        assertTrue(dai.transfer(bob, 100e18));
        assertTrue(weth.transfer(bob, 1e18));
        assertTrue(wbtc.transfer(bob, 1e8));

        usdt.mint(bob, 1e6);
        usdc.mint(bob, 1e6);
        dai.mint(bob, 1e18);
        weth.mint(bob, 1e18);
        wbtc.mint(bob, 1e8);
        vm.stopPrank();

        assertEq(usdt.balanceOf(bob), 101e6);
        assertEq(usdc.balanceOf(bob), 101e6);
        assertEq(dai.balanceOf(bob), 101e18);
        assertEq(weth.balanceOf(bob), 2e18);
        assertEq(wbtc.balanceOf(bob), 2e8);
    }

    function test_mintOnlyOwner() public {
        vm.prank(alice);
        MockUSDC usdc = new MockUSDC(0);

        vm.prank(bob);
        vm.expectRevert(abi.encodeWithSelector(Ownable.OwnableUnauthorizedAccount.selector, bob));
        usdc.mint(bob, 1);

        vm.prank(bob);
        vm.expectRevert(
            abi.encodeWithSelector(IERC20Errors.ERC20InsufficientBalance.selector, bob, uint256(0), uint256(1))
        );
        usdc.transfer(alice, 1);
    }
}
