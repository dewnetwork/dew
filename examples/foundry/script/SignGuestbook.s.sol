// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {Script, console2} from "forge-std/Script.sol";
import {Guestbook} from "../src/Guestbook.sol";

/// @notice Post one message. Env: PRIVATE_KEY, GUESTBOOK, MESSAGE.
contract SignGuestbook is Script {
    function run() external {
        uint256 pk = vm.envUint("PRIVATE_KEY");
        address bookAddr = vm.envAddress("GUESTBOOK");
        string memory message = vm.envOr("MESSAGE", string("signed via forge"));

        Guestbook book = Guestbook(bookAddr);
        vm.startBroadcast(pk);
        uint256 id = book.sign(message);
        vm.stopBroadcast();

        console2.log("Guestbook", bookAddr);
        console2.log("Entry id", id);
        console2.log("Total", book.totalEntries());
    }
}
