package doge

import "errors"

type ChainParams struct {
	ChainName                string
	GenesisBlock             string
	p2pkh_address_prefix     byte
	p2sh_address_prefix      byte
	pkey_prefix              byte
	bip32_privkey_prefix     uint32
	bip32_pubkey_prefix      uint32
	Bip32_WIF_PrivKey_Prefix string
	Bip32_WIF_PubKey_Prefix  string
}

var DogeMainNetChain ChainParams = ChainParams{
	ChainName:                "doge_main",
	GenesisBlock:             "1a91e3dace36e2be3bf030a65679fe821aa1d6ef92e7c9902eb318182c355691",
	p2pkh_address_prefix:     0x1e,       // D
	p2sh_address_prefix:      0x16,       // 9 or A
	pkey_prefix:              0x9e,       // Q or 6
	bip32_privkey_prefix:     0x02fac398, // dgpv
	bip32_pubkey_prefix:      0x02facafd, // dgub
	Bip32_WIF_PrivKey_Prefix: "dgpv",
	Bip32_WIF_PubKey_Prefix:  "dgub",
}

var DogeTestNetChain ChainParams = ChainParams{
	ChainName:                "doge_test",
	GenesisBlock:             "bb0a78264637406b6360aad926284d544d7049f45189db5664f3c4d07350559e",
	p2pkh_address_prefix:     0x71,       // n
	p2sh_address_prefix:      0xc4,       // 2
	pkey_prefix:              0xf1,       // 9 or c
	bip32_privkey_prefix:     0x04358394, // tprv
	bip32_pubkey_prefix:      0x043587cf, // tpub
	Bip32_WIF_PrivKey_Prefix: "tprv",
	Bip32_WIF_PubKey_Prefix:  "tpub",
}

var DogeRegTestChain ChainParams = ChainParams{
	ChainName:                "doge_regtest",
	GenesisBlock:             "3d2160a3b5dc4a9d62e7e66a295f70313ac808440ef7400d6c0772171ce973a5",
	p2pkh_address_prefix:     0x6f,       // n
	p2sh_address_prefix:      0xc4,       // 2
	pkey_prefix:              0xef,       //
	bip32_privkey_prefix:     0x04358394, // tprv
	bip32_pubkey_prefix:      0x043587cf, // tpub
	Bip32_WIF_PrivKey_Prefix: "tprv",
	Bip32_WIF_PubKey_Prefix:  "tpub",
}

// Used in tests only.
var BitcoinMainChain ChainParams = ChainParams{
	ChainName:                "btc_main",
	GenesisBlock:             "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f",
	p2pkh_address_prefix:     0x00,       // 1
	p2sh_address_prefix:      0x05,       // 3
	pkey_prefix:              0x80,       // 5H,5J,5K
	bip32_privkey_prefix:     0x0488ADE4, //
	bip32_pubkey_prefix:      0x0488B21E, //
	Bip32_WIF_PrivKey_Prefix: "xxxx",     // TODO
	Bip32_WIF_PubKey_Prefix:  "xxxx",     // TODO
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

// CAUTION: the result is a best-guess based on the 'version byte' in
// the WIF string. Do not rely on the returned ChainParams alone
// for validation: it will fall back on DogeTestNetChain for unknown
// version bytes (so verify the version byte or bip32-prefix as well)
func ChainFromWIFString(wif string) *ChainParams {
	switch wif[0] {
	case 'D', '9', 'A', 'Q', '6', 'd':
		// FIXME: '9' is ambiguous, check 2nd character over the entire range.
		return &DogeMainNetChain
	case 'n', '2', 'c', 't': // also '9'
		return &DogeTestNetChain
	case '1', '3', '5':
		return &BitcoinMainChain
	default:
		return &DogeTestNetChain
	}
}

// CAUTION: the result is a best-guess based on the 'version byte' in
// the decoded WIF data. Do not rely on the returned ChainParams alone
// for validation: it will fall back on DogeTestNetChain for unknown
// version bytes (so verify the version byte or bip32-prefix as well)
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

func ChainFromGenesisHash(hash string) (*ChainParams, error) {
	if hash == DogeMainNetChain.GenesisBlock {
		return &DogeMainNetChain, nil
	}
	if hash == DogeTestNetChain.GenesisBlock {
		return &DogeTestNetChain, nil
	}
	if hash == DogeRegTestChain.GenesisBlock {
		return &DogeRegTestChain, nil
	}
	return nil, errors.New("ChainFromGenesisHash: unrecognised chain: " + hash)
}
