// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {MockERC20} from "./MockERC20.sol";

/// @title Mock DAI for Dew testnets.
/// @notice 18 decimals like mainnet Dai. Not real DAI — test only.
contract MockDAI is MockERC20 {
    /// @param initialSupply Raw units (18 decimals). e.g. 1_000_000e18 = 1M DAI.
    constructor(uint256 initialSupply) MockERC20("Dai Stablecoin", "DAI", 18, initialSupply) {}
}
