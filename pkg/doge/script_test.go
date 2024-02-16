package doge

import (
	"testing"
)

func TestClassifyScript(t *testing.T) {
	// P2PKH
	pkh_scr := hx2b("76a91454f6fb64f14b756d118a96a57a2f9ebf4b4708fe88ac")
	pkh_adr := Address("DCtMAyy9w2QCrWMRdZ28Kn7GwMfCEp2irP")
	pkh_type, pkh_found := ClassifyScript(pkh_scr, &DogeMainNetChain)
	if pkh_type != ScriptTypeP2PKH {
		t.Errorf("Wrong script type: %v vs %v", pkh_type, ScriptTypeP2PKH)
	}
	if pkh_found != pkh_adr {
		t.Errorf("Wrong address: %v vs %v", pkh_found, pkh_adr)
	}
	// P2SH
	sh_scr := hx2b("a9149feb23c522d5404c1974b761b4079d06e485325387")
	sh_adr := Address("A71qnghzaMDGNboHC9XkDu7mmKjdQNLqiA")
	sh_type, sh_found := ClassifyScript(sh_scr, &DogeMainNetChain)
	if sh_type != ScriptTypeP2SH {
		t.Errorf("Wrong script type: %v vs %v", sh_type, ScriptTypeP2SH)
	}
	if sh_found != sh_adr {
		t.Errorf("Wrong address: %v vs %v", sh_found, sh_adr)
	}
	// P2PK uncompressed
	pku_scr := hx2b("4104ae1a62fe09c5f51b13905f07f06b99a2f7159b2225f374cd378d71302fa28414e7aab37397f554a7df5f142c21c1b7303b8a0626f1baded5c72a704f7e6cd84cac")
	pku_type, pku_found := ClassifyScript(pku_scr, &DogeMainNetChain)
	if pku_type != ScriptTypeP2PK {
		t.Errorf("Wrong script type: %v vs %v", pku_type, ScriptTypeP2PK)
	}
	if pku_found != "" {
		t.Errorf("Shouldn't generate Base58 Address: %v", pku_found)
	}
	// P2PK compressed
	pkc_scr := hx2b("2102ae1a62fe09c5f51b13905f07f06b99a2f7159b2225f374cd378d71302fa28414ac")
	pkc_type, pkc_found := ClassifyScript(pkc_scr, &DogeMainNetChain)
	if pkc_type != ScriptTypeP2PK {
		t.Errorf("Wrong script type: %v vs %v", pkc_type, ScriptTypeP2PK)
	}
	if pkc_found != "" {
		t.Errorf("Shouldn't generate Base58 Address: %v", pkc_found)
	}
	// NullData
	nud_scr := hx2b("6a1454f6fb64f14b756d118a96a57a2f9ebf4b4708fe")
	nud_type, nud_found := ClassifyScript(nud_scr, &DogeMainNetChain)
	if nud_type != ScriptTypeNullData {
		t.Errorf("Wrong script type: %v vs %v", nud_type, ScriptTypeNullData)
	}
	if nud_found != "" {
		t.Errorf("Shouldn't generate Base58 Address: %v", nud_found)
	}
	// MultiSig
	ms_scr := hx2b("514104cc71eb30d653c0c3163990c47b976f3fb3f37cccdcbedb169a1dfef58bbfbfaff7d8a473e7e2e6d317b87bafe8bde97e3cf8f065dec022b51d11fcdd0d348ac4410461cbdcc5409fb4b4d42b51d33381354d80e550078cb532a34bfa2fcfdeb7d76519aecc62770f5b0e4ef8551946d8a540911abe3e7854a26f39f58b25c15342af52ae")
	ms_type, ms_found := ClassifyScript(ms_scr, &DogeMainNetChain)
	if ms_type != ScriptTypeMultiSig {
		t.Errorf("Wrong script type: %v vs %v", ms_type, ScriptTypeMultiSig)
	}
	if ms_found != "" {
		t.Errorf("Shouldn't generate Base58 Addresses: %v", ms_found)
	}
}
