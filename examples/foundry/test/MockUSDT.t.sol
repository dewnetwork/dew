// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {Test} from "forge-std/Test.sol";
import {IERC20Errors} from "@openzeppelin/contracts/interfaces/draft-IERC6093.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";
import {MockUSDT} from "../src/MockUSDT.sol";

contract MockUSDTTest is Test {
    MockUSDT internal usdt;
    address internal alice = address(0xA11CE);
    address internal bob = address(0xB0B);
    address internal spender = address(0x51E1D);

    uint256 internal constant SUPPLY = 1_000_000 * 1e6;

    function setUp() public {
        vm.prank(alice);
        usdt = new MockUSDT(SUPPLY);
    }

    function test_metadata() public view {
        assertEq(usdt.name(), "Tether USD");
        assertEq(usdt.symbol(), "USDT");
        assertEq(usdt.decimals(), 6);
        assertEq(usdt.totalSupply(), SUPPLY);
        assertEq(usdt.owner(), alice);
    }

    function test_initialSupply() public view {
        assertEq(usdt.balanceOf(alice), SUPPLY);
        assertEq(usdt.balanceOf(bob), 0);
    }

    function test_transfer() public {
        uint256 amount = 100 * 1e6;
        vm.prank(alice);
        bool ok = usdt.transfer(bob, amount);
        assertTrue(ok);
        assertEq(usdt.balanceOf(alice), SUPPLY - amount);
        assertEq(usdt.balanceOf(bob), amount);
    }

    function test_transferRevertsOnInsufficientBalance() public {
        vm.prank(bob);
        vm.expectRevert(
            abi.encodeWithSelector(IERC20Errors.ERC20InsufficientBalance.selector, bob, uint256(0), uint256(1))
        );
        usdt.transfer(alice, 1);
    }

    function test_approveAndTransferFrom() public {
        uint256 amount = 50 * 1e6;
        vm.prank(alice);
        usdt.approve(spender, amount);

        vm.prank(spender);
        bool ok = usdt.transferFrom(alice, bob, amount);
        assertTrue(ok);
        assertEq(usdt.balanceOf(bob), amount);
        assertEq(usdt.allowance(alice, spender), 0);
    }

    function test_mintOnlyOwner() public {
        uint256 extra = 10 * 1e6;
        vm.prank(alice);
        usdt.mint(bob, extra);
        assertEq(usdt.balanceOf(bob), extra);
        assertEq(usdt.totalSupply(), SUPPLY + extra);

        vm.prank(bob);
        vm.expectRevert(abi.encodeWithSelector(Ownable.OwnableUnauthorizedAccount.selector, bob));
        usdt.mint(bob, 1);
    }

    function test_transferOwnership() public {
        vm.prank(alice);
        usdt.transferOwnership(bob);
        assertEq(usdt.owner(), bob);

        vm.prank(bob);
        usdt.mint(bob, 1e6);
        assertEq(usdt.balanceOf(bob), 1e6);
    }
}
