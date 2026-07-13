// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {Script, console2} from "forge-std/Script.sol";
import {Guestbook} from "../src/Guestbook.sol";

/// @notice Post two messages in one script (two nonces).
/// @dev On Dew auto-mine, if both are ready they may land in one multi-tx block (C1 pack).
///      Env: PRIVATE_KEY, GUESTBOOK; optional MESSAGE_A / MESSAGE_B.
contract BatchSignGuestbook is Script {
    function run() external {
        uint256 pk = vm.envUint("PRIVATE_KEY");
        address bookAddr = vm.envAddress("GUESTBOOK");
        string memory a = vm.envOr("MESSAGE_A", string("batch #1"));
        string memory b = vm.envOr("MESSAGE_B", string("batch #2"));

        Guestbook book = Guestbook(bookAddr);
        vm.startBroadcast(pk);
        uint256 id0 = book.sign(a);
        uint256 id1 = book.sign(b);
        vm.stopBroadcast();

        console2.log("Guestbook", bookAddr);
        console2.log("Entry ids", id0, id1);
        console2.log("Total", book.totalEntries());
    }
}
