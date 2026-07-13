// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {MockERC20} from "./MockERC20.sol";

/// @title Mock WETH for Dew testnets.
/// @notice 18-decimal mintable stand-in for Wrapped Ether. No deposit/withdraw of native DEW.
/// @dev Prefer owner `mint` for balances; apps that need wrap semantics should add a local wrapper.
contract MockWETH is MockERC20 {
    /// @param initialSupply Raw units (18 decimals). e.g. 10_000e18 = 10k WETH.
    constructor(uint256 initialSupply) MockERC20("Wrapped Ether", "WETH", 18, initialSupply) {}
}
