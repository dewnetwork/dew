// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

/// @title Minimal ERC-20 for Dew DX samples (Foundry / MetaMask / public testnet).
/// @notice Same surface as the in-repo Go fixture (`core/vm` Token): balanceOf, transfer, Transfer.
contract Token {
    string public constant name = "Dew Sample Token";
    string public constant symbol = "DST";
    uint8 public constant decimals = 18;

    mapping(address => uint256) public balanceOf;

    event Transfer(address indexed from, address indexed to, uint256 value);

    constructor(uint256 initialSupply) {
        balanceOf[msg.sender] = initialSupply;
        emit Transfer(address(0), msg.sender, initialSupply);
    }

    function transfer(address to, uint256 amount) external returns (bool) {
        require(balanceOf[msg.sender] >= amount, "balance");
        unchecked {
            balanceOf[msg.sender] -= amount;
            balanceOf[to] += amount;
        }
        emit Transfer(msg.sender, to, amount);
        return true;
    }
}
