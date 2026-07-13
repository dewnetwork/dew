// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {Test} from "forge-std/Test.sol";
import {Token} from "../src/Token.sol";

contract TokenTest is Test {
    Token internal token;
    address internal alice = address(0xA11CE);
    address internal bob = address(0xB0B);

    uint256 internal constant SUPPLY = 1_000_000 ether;

    function setUp() public {
        vm.prank(alice);
        token = new Token(SUPPLY);
    }

    function test_initialSupply() public view {
        assertEq(token.balanceOf(alice), SUPPLY);
        assertEq(token.balanceOf(bob), 0);
    }

    function test_transfer() public {
        vm.prank(alice);
        bool ok = token.transfer(bob, 100 ether);
        assertTrue(ok);
        assertEq(token.balanceOf(alice), SUPPLY - 100 ether);
        assertEq(token.balanceOf(bob), 100 ether);
    }

    function test_transferRevertsOnInsufficientBalance() public {
        vm.prank(bob);
        vm.expectRevert(bytes("balance"));
        token.transfer(alice, 1);
    }
}
