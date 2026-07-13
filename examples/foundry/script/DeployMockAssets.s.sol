// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {Script, console2} from "forge-std/Script.sol";
import {MockUSDT} from "../src/MockUSDT.sol";
import {MockUSDC} from "../src/MockUSDC.sol";
import {MockDAI} from "../src/MockDAI.sol";
import {MockWETH} from "../src/MockWETH.sol";
import {MockWBTC} from "../src/MockWBTC.sol";

/// @notice Deploy the full mock asset basket for Dew local / public-testnet-v1.
/// @dev USDT + USDC (6) · DAI + WETH (18) · WBTC (8). Not real assets.
///
/// Optional env:
///   TRANSFER_TO — address receives a small demo amount of each token
contract DeployMockAssets is Script {
    uint256 internal constant USDT_SUPPLY = 1_000_000 * 1e6;
    uint256 internal constant USDC_SUPPLY = 1_000_000 * 1e6;
    uint256 internal constant DAI_SUPPLY = 1_000_000 * 1e18;
    uint256 internal constant WETH_SUPPLY = 10_000 * 1e18;
    uint256 internal constant WBTC_SUPPLY = 100 * 1e8;

    uint256 internal constant USDT_DEMO = 1_000 * 1e6;
    uint256 internal constant USDC_DEMO = 1_000 * 1e6;
    uint256 internal constant DAI_DEMO = 1_000 * 1e18;
    uint256 internal constant WETH_DEMO = 10 * 1e18;
    uint256 internal constant WBTC_DEMO = 1 * 1e8;

    function run() external {
        uint256 pk = vm.envUint("PRIVATE_KEY");
        address deployer = vm.addr(pk);
        address recipient = vm.envOr("TRANSFER_TO", address(0));

        vm.startBroadcast(pk);

        MockUSDT usdt = new MockUSDT(USDT_SUPPLY);
        MockUSDC usdc = new MockUSDC(USDC_SUPPLY);
        MockDAI dai = new MockDAI(DAI_SUPPLY);
        MockWETH weth = new MockWETH(WETH_SUPPLY);
        MockWBTC wbtc = new MockWBTC(WBTC_SUPPLY);

        console2.log("Deployer", deployer);
        console2.log("--- Mock assets (test only) ---");
        console2.log("USDT", address(usdt));
        console2.log("  decimals", uint256(usdt.decimals()));
        console2.log("  supply  ", USDT_SUPPLY);
        console2.log("USDC", address(usdc));
        console2.log("  decimals", uint256(usdc.decimals()));
        console2.log("  supply  ", USDC_SUPPLY);
        console2.log("DAI ", address(dai));
        console2.log("  decimals", uint256(dai.decimals()));
        console2.log("  supply  ", DAI_SUPPLY);
        console2.log("WETH", address(weth));
        console2.log("  decimals", uint256(weth.decimals()));
        console2.log("  supply  ", WETH_SUPPLY);
        console2.log("WBTC", address(wbtc));
        console2.log("  decimals", uint256(wbtc.decimals()));
        console2.log("  supply  ", WBTC_SUPPLY);

        if (recipient != address(0)) {
            require(usdt.transfer(recipient, USDT_DEMO), "usdt transfer");
            require(usdc.transfer(recipient, USDC_DEMO), "usdc transfer");
            require(dai.transfer(recipient, DAI_DEMO), "dai transfer");
            require(weth.transfer(recipient, WETH_DEMO), "weth transfer");
            require(wbtc.transfer(recipient, WBTC_DEMO), "wbtc transfer");
            console2.log("Demo amounts transferred to", recipient);
        }

        console2.log("--- MetaMask import ---");
        console2.log("symbol USDT decimals 6  address above");
        console2.log("symbol USDC decimals 6  address above");
        console2.log("symbol DAI  decimals 18 address above");
        console2.log("symbol WETH decimals 18 address above");
        console2.log("symbol WBTC decimals 8  address above");
        console2.log(
            "Explorer known tokens: PUBLIC_KNOWN_TOKENS=USDT:0x..,USDC:0x..,DAI:0x..,WETH:0x..,WBTC:0x.."
        );

        vm.stopBroadcast();
    }
}
