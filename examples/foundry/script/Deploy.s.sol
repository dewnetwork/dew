// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {Script, console2} from "forge-std/Script.sol";
import {Token} from "../src/Token.sol";

/// @notice Deploy Token and optionally transfer a slice to a second address.
/// @dev Local: Anvil #0 key (see README). Public: fund via faucet first.
contract Deploy is Script {
    uint256 internal constant SUPPLY = 1_000_000 ether;
    uint256 internal constant DEMO_TRANSFER = 1_000 ether;

    function run() external {
        uint256 pk = vm.envUint("PRIVATE_KEY");
        address deployer = vm.addr(pk);
        address recipient = vm.envOr("TRANSFER_TO", address(0));

        vm.startBroadcast(pk);
        Token token = new Token(SUPPLY);
        console2.log("Token", address(token));
        console2.log("Deployer", deployer);
        console2.log("Supply", SUPPLY);

        if (recipient != address(0)) {
            require(token.transfer(recipient, DEMO_TRANSFER), "transfer failed");
            console2.log("Transferred", DEMO_TRANSFER);
            console2.log("To", recipient);
        }
        vm.stopBroadcast();
    }
}
