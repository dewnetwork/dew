// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

/// @title Configurable mock ERC-20 for Dew testnets (local + public-testnet-v1).
/// @notice OpenZeppelin ERC-20 with fixed decimals + owner mint. Test only — not real assets.
/// @dev Prefer the named wrappers (MockUSDT, MockUSDC, …) for fixed metadata.
contract MockERC20 is ERC20, Ownable {
    uint8 private immutable _decimals;

    /// @param name_ Token name (e.g. "USD Coin").
    /// @param symbol_ Token symbol (e.g. "USDC").
    /// @param decimals_ Decimals (6 for stables, 18 for WETH/DAI, 8 for WBTC).
    /// @param initialSupply Raw units minted to deployer (already scaled by decimals).
    constructor(
        string memory name_,
        string memory symbol_,
        uint8 decimals_,
        uint256 initialSupply
    ) ERC20(name_, symbol_) Ownable(msg.sender) {
        _decimals = decimals_;
        if (initialSupply > 0) {
            _mint(msg.sender, initialSupply);
        }
    }

    function decimals() public view override returns (uint8) {
        return _decimals;
    }

    /// @notice Mint test tokens (owner only). Amount is raw units at this token's decimals.
    function mint(address to, uint256 amount) external onlyOwner {
        _mint(to, amount);
    }
}
