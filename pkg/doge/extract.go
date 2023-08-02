package doge

// Given a Bip32 Extended Private Key WIF, extract the WIF-encoded EC Private Key.
func ExtractECPrivKeyFromBip32(ext_key string) (string, error) {
	bkey, err := DecodeBip32WIF(ext_key, nil)
	if err != nil {
		return "", err
	}
	priv, err := bkey.GetECPrivKey()
	if err != nil {
		return "", err
	}
	chain := ChainFromKeyBits(bkey.keyType)
	return EncodeECPrivKeyWIF(priv, chain), nil
}
