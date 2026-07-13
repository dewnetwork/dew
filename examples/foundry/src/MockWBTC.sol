// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {MockERC20} from "./MockERC20.sol";

/// @title Mock WBTC for Dew testnets.
/// @notice 8 decimals like mainnet Wrapped Bitcoin. Not real WBTC — test only.
contract MockWBTC is MockERC20 {
    /// @param initialSupply Raw units (8 decimals). e.g. 100e8 = 100 WBTC.
    constructor(uint256 initialSupply) MockERC20("Wrapped BTC", "WBTC", 8, initialSupply) {}
}
