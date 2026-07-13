// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {Test} from "forge-std/Test.sol";
import {Guestbook} from "../src/Guestbook.sol";

contract GuestbookTest is Test {
    Guestbook internal book;
    address internal alice = address(0xA11CE);

    function setUp() public {
        book = new Guestbook();
    }

    function test_signAndRead() public {
        vm.prank(alice);
        uint256 id = book.sign("hello dew");
        assertEq(id, 0);
        assertEq(book.totalEntries(), 1);

        (address author, uint64 ts, string memory msg_) = book.getEntry(0);
        assertEq(author, alice);
        assertGt(uint256(ts), 0);
        assertEq(msg_, "hello dew");
    }

    function test_rejectEmpty() public {
        vm.expectRevert(bytes("empty"));
        book.sign("");
    }

    function test_rejectTooLong() public {
        // 281 bytes of 'a'
        bytes memory buf = new bytes(281);
        for (uint256 i = 0; i < 281; i++) {
            buf[i] = "a";
        }
        vm.expectRevert(bytes("too long"));
        book.sign(string(buf));
    }

    function test_multipleAuthors() public {
        address bob = address(0xB0B);
        vm.prank(alice);
        book.sign("from alice");
        vm.prank(bob);
        book.sign("from bob");
        assertEq(book.totalEntries(), 2);
        (address a,,) = book.getEntry(0);
        (address b,,) = book.getEntry(1);
        assertEq(a, alice);
        assertEq(b, bob);
    }
}
