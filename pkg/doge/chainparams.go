package doge

type ChainParams struct {
	p2pkh_address_prefix byte
	p2sh_address_prefix  byte
	pkey_prefix          byte
	bip32_privkey_prefix uint32
	bip32_pubkey_prefix  uint32
}

var MainChain ChainParams = ChainParams{
	p2pkh_address_prefix: 0x1e,       // D
	p2sh_address_prefix:  0x16,       // 9 or A
	pkey_prefix:          0x9e,       // Q or 6
	bip32_privkey_prefix: 0x02fac398, // dgpv
	bip32_pubkey_prefix:  0x02facafd, // dgub
}

var TestChain ChainParams = ChainParams{
	p2pkh_address_prefix: 0x71,       // n
	p2sh_address_prefix:  0xc4,       // 2
	pkey_prefix:          0xf1,       // 9 or c
	bip32_privkey_prefix: 0x04358394, // tprv
	bip32_pubkey_prefix:  0x043587cf, // tpub
}

var RegTestChain ChainParams = ChainParams{
	p2pkh_address_prefix: 0x6f,       //
	p2sh_address_prefix:  0xc4,       // 2
	pkey_prefix:          0xef,       //
	bip32_privkey_prefix: 0x04358394, // tprv
	bip32_pubkey_prefix:  0x043587cf, // tpub
}

var BitcoinMainChain ChainParams = ChainParams{
	p2pkh_address_prefix: 0x00,       // 1
	p2sh_address_prefix:  0x05,       // 3
	pkey_prefix:          0x80,       // 5H,5J,5K
	bip32_privkey_prefix: 0x0488ADE4, //
	bip32_pubkey_prefix:  0x0488B21E, //
}

func ChainFromTestNetFlag(is_testnet bool) *ChainParams {
	if is_testnet {
		return &TestChain
	}
	return &MainChain
}

func ChainFromWIFPrefix(bytes []byte) *ChainParams {
	if len(bytes) == 0 {
		return &TestChain
	}
	switch bytes[0] {
	case 0x1e, 0x16, 0x9e, 0x02:
		return &MainChain
	default:
		return &TestChain
	}
}

func ChainFromBip32Version(version uint32) *ChainParams {
	switch version {
	case 0x02fac398, 0x02facafd: // dgpv, dgub
		return &MainChain
	default:
		return &TestChain
	}
}
