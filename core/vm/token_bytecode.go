package vm

// TokenCreationBytecode is solc 0.8.24 optimized bytecode for:
//
//	contract Token {
//	  mapping(address => uint256) public balanceOf;
//	  event Transfer(address indexed from, address indexed to, uint256 value);
//	  constructor(uint256 supply) { balanceOf[msg.sender] = supply; emit Transfer(address(0), msg.sender, supply); }
//	  function transfer(address to, uint256 amount) external returns (bool);
//	}
//
// Constructor takes one uint256 argument (appended ABI-encoded by helpers).
const TokenCreationBytecode = "608060405234801561000f575f80fd5b5060405161027338038061027383398101604081905261002e91610075565b335f81815260208181526040808320859055518481527fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef910160405180910390a35061008c565b5f60208284031215610085575f80fd5b5051919050565b6101da806100995f395ff3fe608060405234801561000f575f80fd5b5060043610610034575f3560e01c806370a0823114610038578063a9059cbb1461006a575b5f80fd5b61005761004636600461015c565b5f6020819052908152604090205481565b6040519081526020015b60405180910390f35b61007d61007836600461017c565b61008d565b6040519015158152602001610061565b335f908152602081905260408120548211156100d95760405162461bcd60e51b815260206004820152600760248201526662616c616e636560c81b604482015260640160405180910390fd5b335f81815260208181526040808320805487900390556001600160a01b03871680845292819020805487019055518581529192917fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef910160405180910390a350600192915050565b80356001600160a01b0381168114610157575f80fd5b919050565b5f6020828403121561016c575f80fd5b61017582610141565b9392505050565b5f806040838503121561018d575f80fd5b61019683610141565b94602093909301359350505056fea264697066735822122098835b6600b46d3e44e1145030109088288d1c1aa9cdaf7e6c5b7505738177fc64736f6c63430008180033"

// TokenABI JSON for packing calls (balanceOf / transfer / constructor).
const TokenABI = `[
  {"inputs":[{"internalType":"uint256","name":"supply","type":"uint256"}],"stateMutability":"nonpayable","type":"constructor"},
  {"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"from","type":"address"},{"indexed":true,"internalType":"address","name":"to","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"}],"name":"Transfer","type":"event"},
  {"inputs":[{"internalType":"address","name":"","type":"address"}],"name":"balanceOf","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"transfer","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"}
]`
