package doge

type ChainParams struct {
	p2pkh_address_prefix byte
	p2sh_address_prefix  byte
	pkey_prefix          byte
	bip32_privkey_prefix []byte
	bip32_pubkey_prefix  []byte
}

var MainChain ChainParams = ChainParams{
	p2pkh_address_prefix: 0x1e,                           // D
	p2sh_address_prefix:  0x16,                           // 9 or A
	pkey_prefix:          0x9e,                           // Q or 6
	bip32_privkey_prefix: []byte{0x02, 0xfa, 0xc3, 0x98}, // dgpv
	bip32_pubkey_prefix:  []byte{0x02, 0xfa, 0xca, 0xfd}, // dgub
}

var TestChain ChainParams = ChainParams{
	p2pkh_address_prefix: 0x71,                           // n
	p2sh_address_prefix:  0xc4,                           // 2
	pkey_prefix:          0xf1,                           // 9 or c
	bip32_privkey_prefix: []byte{0x04, 0x35, 0x83, 0x94}, // tprv
	bip32_pubkey_prefix:  []byte{0x04, 0x35, 0x87, 0xcf}, // tpub
}

var RegTestChain ChainParams = ChainParams{
	p2pkh_address_prefix: 0x6f,                           //
	p2sh_address_prefix:  0xc4,                           // 2
	pkey_prefix:          0xef,                           //
	bip32_privkey_prefix: []byte{0x04, 0x35, 0x83, 0x94}, // tprv
	bip32_pubkey_prefix:  []byte{0x04, 0x35, 0x87, 0xcf}, // tpub
}

var BitcoinMainChain ChainParams = ChainParams{
	p2pkh_address_prefix: 0x00,                           // 1
	p2sh_address_prefix:  0x05,                           // 3
	pkey_prefix:          0x80,                           // 5H,5J,5K
	bip32_privkey_prefix: []byte{0x00, 0x00, 0x00, 0x00}, // todo
	bip32_pubkey_prefix:  []byte{0x00, 0x00, 0x00, 0x00}, // todo
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
