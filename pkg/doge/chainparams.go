package doge

type ChainParams struct {
	p2pkh_address_prefix byte
	p2sh_address_prefix  byte
	pkey_prefix          byte
	bip32_privkey_prefix uint32
	bip32_pubkey_prefix  uint32
}

var DogeMainNetChain ChainParams = ChainParams{
	p2pkh_address_prefix: 0x1e,       // D
	p2sh_address_prefix:  0x16,       // 9 or A
	pkey_prefix:          0x9e,       // Q or 6
	bip32_privkey_prefix: 0x02fac398, // dgpv
	bip32_pubkey_prefix:  0x02facafd, // dgub
}

var DogeTestNetChain ChainParams = ChainParams{
	p2pkh_address_prefix: 0x71,       // n
	p2sh_address_prefix:  0xc4,       // 2
	pkey_prefix:          0xf1,       // 9 or c
	bip32_privkey_prefix: 0x04358394, // tprv
	bip32_pubkey_prefix:  0x043587cf, // tpub
}

var DogeRegTestChain ChainParams = ChainParams{
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

func ChainFromTestNetFlag(isTestNet bool) *ChainParams {
	if isTestNet {
		return &DogeTestNetChain
	}
	return &DogeMainNetChain
}

func ChainFromKeyBits(keyType KeyBits) *ChainParams {
	if (keyType & mainNetDoge) != 0 {
		return &DogeMainNetChain
	}
	if (keyType & mainNetBtc) != 0 {
		return &BitcoinMainChain
	}
	return &DogeTestNetChain // fallback
}

func KeyBitsForChain(chain *ChainParams) KeyBits {
	bits := 0
	if chain == &DogeMainNetChain {
		bits |= mainNetDoge
	} else if chain == &DogeTestNetChain {
		bits |= testNetDoge
	} else if chain == &BitcoinMainChain {
		bits |= mainNetBtc
	}
	return bits
}

func ChainFromWIFPrefix(bytes []byte, allowNonDoge bool) *ChainParams {
	if len(bytes) == 0 {
		return &DogeTestNetChain // fallback
	}
	switch bytes[0] {
	case 0x1e, 0x16, 0x9e, 0x02:
		return &DogeMainNetChain
	case 0x71, 0xc4, 0xf1:
		return &DogeTestNetChain
	case 0x04:
		if allowNonDoge {
			// 0x04 is ambigous (DogeTestNetChain vs BitcoinMainChain)
			if len(bytes) > 1 && bytes[1] == 0x88 {
				return &BitcoinMainChain
			}
		}
		return &DogeTestNetChain
	case 0x6f, 0xef:
		return &DogeRegTestChain
	case 0x00, 0x05, 0x80:
		if allowNonDoge {
			return &BitcoinMainChain
		}
	}
	return &DogeTestNetChain // fallback
}

func ChainFromBip32Version(version uint32, allowNonDoge bool) *ChainParams {
	switch version {
	case 0x02fac398, 0x02facafd: // dgpv, dgub
		return &DogeMainNetChain
	case 0x04358394, 0x043587cf: // tprv, tpub
		return &DogeMainNetChain
	case 0x0488ADE4, 0x0488B21E: // bitcoin mainnet
		if allowNonDoge {
			return &BitcoinMainChain
		}
	}
	return &DogeTestNetChain // fallback
}
