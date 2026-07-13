// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {Script, console2} from "forge-std/Script.sol";
import {MockUSDT} from "../src/MockUSDT.sol";

/// @notice Deploy MockUSDT (6 decimals) for Dew local / public-testnet-v1.
/// @dev Local: Anvil #0 key. Public: faucet-funded PRIVATE_KEY.
///
/// Optional env:
///   TRANSFER_TO — address to receive DEMO_TRANSFER
///   MINT_TO     — address for extra owner mint (MINT_AMOUNT raw units)
///   MINT_AMOUNT — raw 6-decimal units (default 0)
contract DeployUSDT is Script {
    /// 1_000_000 USDT with 6 decimals
    uint256 internal constant SUPPLY = 1_000_000 * 1e6;
    /// 1_000 USDT demo transfer
    uint256 internal constant DEMO_TRANSFER = 1_000 * 1e6;

    function run() external {
        uint256 pk = vm.envUint("PRIVATE_KEY");
        address deployer = vm.addr(pk);
        address recipient = vm.envOr("TRANSFER_TO", address(0));
        address mintTo = vm.envOr("MINT_TO", address(0));
        uint256 mintAmount = vm.envOr("MINT_AMOUNT", uint256(0));

        vm.startBroadcast(pk);
        MockUSDT usdt = new MockUSDT(SUPPLY);
        console2.log("MockUSDT", address(usdt));
        console2.log("Deployer", deployer);
        console2.log("Supply (raw 6 dec)", SUPPLY);
        console2.log("name", usdt.name());
        console2.log("symbol", usdt.symbol());
        console2.log("decimals", uint256(usdt.decimals()));

        if (recipient != address(0)) {
            require(usdt.transfer(recipient, DEMO_TRANSFER), "transfer failed");
            console2.log("Transferred", DEMO_TRANSFER);
            console2.log("To", recipient);
        }

        if (mintTo != address(0) && mintAmount > 0) {
            usdt.mint(mintTo, mintAmount);
            console2.log("Minted", mintAmount);
            console2.log("MintTo", mintTo);
        }
        vm.stopBroadcast();
    }
}
