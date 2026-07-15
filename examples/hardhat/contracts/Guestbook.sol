// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

/// @title On-chain guestbook for Dew public-testnet / local devnet demos.
/// @notice Append-only messages with P3c replies and emoji reactions (redeploy required for ABI).
contract Guestbook {
    uint256 public constant MAX_MESSAGE_BYTES = 280;
    /// @notice parentId value for root posts from sign().
    uint256 public constant PARENT_NONE = type(uint256).max;
    /// @notice Valid reaction kinds are 0..MAX_REACTION_KIND inclusive.
    uint8 public constant MAX_REACTION_KIND = 3;

    struct Entry {
        address author;
        uint64 timestamp;
        string message;
        uint256 parentId;
    }

    Entry[] private _entries;

    // entryId => kind => count
    mapping(uint256 => mapping(uint8 => uint256)) private _reactionCounts;
    // entryId => kind => reactor => active
    mapping(uint256 => mapping(uint8 => mapping(address => bool))) private _reacted;

    event Signed(uint256 indexed id, address indexed author, string message, uint256 parentId);
    event Reacted(uint256 indexed entryId, address indexed reactor, uint8 kind, bool active);

    /// @notice Number of messages posted.
    function totalEntries() external view returns (uint256) {
        return _entries.length;
    }

    /// @notice Read one entry by id (0 .. totalEntries-1).
    function getEntry(uint256 id)
        external
        view
        returns (address author, uint64 timestamp, string memory message, uint256 parentId)
    {
        require(id < _entries.length, "id");
        Entry storage e = _entries[id];
        return (e.author, e.timestamp, e.message, e.parentId);
    }

    /// @notice Reaction count for a kind on an entry.
    function reactionCount(uint256 entryId, uint8 kind) external view returns (uint256) {
        return _reactionCounts[entryId][kind];
    }

    /// @notice Whether `who` currently has reaction `kind` on `entryId`.
    function hasReacted(uint256 entryId, address who, uint8 kind) external view returns (bool) {
        return _reacted[entryId][kind][who];
    }

    /// @notice Post a root message (max 280 UTF-8 bytes as measured by `bytes(message).length`).
    function sign(string calldata message) external returns (uint256 id) {
        return _post(PARENT_NONE, message);
    }

    /// @notice Reply to an existing entry.
    function reply(uint256 parentId, string calldata message) external returns (uint256 id) {
        require(parentId < _entries.length, "parent");
        return _post(parentId, message);
    }

    /// @notice Toggle a reaction kind (0..3) on an entry for msg.sender.
    function react(uint256 entryId, uint8 kind) external {
        require(entryId < _entries.length, "id");
        require(kind <= MAX_REACTION_KIND, "kind");
        bool was = _reacted[entryId][kind][msg.sender];
        if (was) {
            _reacted[entryId][kind][msg.sender] = false;
            _reactionCounts[entryId][kind] -= 1;
            emit Reacted(entryId, msg.sender, kind, false);
        } else {
            _reacted[entryId][kind][msg.sender] = true;
            _reactionCounts[entryId][kind] += 1;
            emit Reacted(entryId, msg.sender, kind, true);
        }
    }

    function _post(uint256 parentId, string calldata message) internal returns (uint256 id) {
        require(bytes(message).length > 0, "empty");
        require(bytes(message).length <= MAX_MESSAGE_BYTES, "too long");
        id = _entries.length;
        _entries.push(
            Entry({author: msg.sender, timestamp: uint64(block.timestamp), message: message, parentId: parentId})
        );
        emit Signed(id, msg.sender, message, parentId);
    }
}
