// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {Script, console2} from "forge-std/Script.sol";
import {Guestbook} from "../src/Guestbook.sol";

/// @notice Deploy Guestbook. Optional first message via MESSAGE env.
contract DeployGuestbook is Script {
    function run() external {
        uint256 pk = vm.envUint("PRIVATE_KEY");
        string memory message = vm.envOr("MESSAGE", string("Hello Dew public-testnet-v1"));

        vm.startBroadcast(pk);
        Guestbook book = new Guestbook();
        console2.log("Guestbook", address(book));

        if (bytes(message).length > 0) {
            uint256 id = book.sign(message);
            console2.log("First entry id", id);
            console2.log("Message", message);
        }
        vm.stopBroadcast();
    }
}
