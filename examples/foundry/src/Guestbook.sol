// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

/// @title On-chain guestbook for Dew public-testnet / local devnet demos.
/// @notice Anyone can post a short message; entries are append-only and indexed by id.
contract Guestbook {
    uint256 public constant MAX_MESSAGE_BYTES = 280;

    struct Entry {
        address author;
        uint64 timestamp;
        string message;
    }

    Entry[] private _entries;

    event Signed(uint256 indexed id, address indexed author, string message);

    /// @notice Number of messages posted.
    function totalEntries() external view returns (uint256) {
        return _entries.length;
    }

    /// @notice Read one entry by id (0 .. totalEntries-1).
    function getEntry(uint256 id) external view returns (address author, uint64 timestamp, string memory message) {
        require(id < _entries.length, "id");
        Entry storage e = _entries[id];
        return (e.author, e.timestamp, e.message);
    }

    /// @notice Post a message (max 280 UTF-8 bytes as measured by `bytes(message).length`).
    function sign(string calldata message) external returns (uint256 id) {
        require(bytes(message).length > 0, "empty");
        require(bytes(message).length <= MAX_MESSAGE_BYTES, "too long");
        id = _entries.length;
        _entries.push(Entry({author: msg.sender, timestamp: uint64(block.timestamp), message: message}));
        emit Signed(id, msg.sender, message);
    }
}
