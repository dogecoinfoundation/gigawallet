package doge

import (
	"testing"
)

func TestClassifyScript(t *testing.T) {
	// P2PKH
	pkh_scr := hx2b("76a91454f6fb64f14b756d118a96a57a2f9ebf4b4708fe88ac")
	pkh_adr := Address("DCtMAyy9w2QCrWMRdZ28Kn7GwMfCEp2irP")
	pkh_type, pkh_arr := ClassifyScript(pkh_scr, &DogeMainNetChain)
	if pkh_type != ScriptTypeP2PKH {
		t.Errorf("Wrong script type: %v vs %v", pkh_type, ScriptTypeP2PKH)
	}
	if len(pkh_arr) != 1 || pkh_arr[0] != pkh_adr {
		t.Errorf("Wrong address: %v vs %v", pkh_arr[0], pkh_adr)
	}
	// P2SH
	sh_scr := hx2b("a9149feb23c522d5404c1974b761b4079d06e485325387")
	sh_adr := Address("A71qnghzaMDGNboHC9XkDu7mmKjdQNLqiA")
	sh_type, sh_arr := ClassifyScript(sh_scr, &DogeMainNetChain)
	if sh_type != ScriptTypeP2SH {
		t.Errorf("Wrong script type: %v vs %v", sh_type, ScriptTypeP2SH)
	}
	if len(sh_arr) != 1 || sh_arr[0] != sh_adr {
		t.Errorf("Wrong address: %v vs %v", sh_arr[0], sh_adr)
	}
	// P2PK uncompressed
	pku_scr := hx2b("4104ae1a62fe09c5f51b13905f07f06b99a2f7159b2225f374cd378d71302fa28414e7aab37397f554a7df5f142c21c1b7303b8a0626f1baded5c72a704f7e6cd84cac")
	pku_adr := Address("26bnyi8VtSMREjx6Pntsppgrwb9YZRzMw78")
	pku_type, pku_arr := ClassifyScript(pku_scr, &DogeMainNetChain)
	if pku_type != ScriptTypeP2PK {
		t.Errorf("Wrong script type: %v vs %v", pku_type, ScriptTypeP2PK)
	}
	if len(pku_arr) != 1 || pku_arr[0] != pku_adr {
		t.Errorf("Wrong address: %v vs %v", pku_arr[0], pku_adr)
	}
	// P2PK compressed
	pkc_scr := hx2b("2102ae1a62fe09c5f51b13905f07f06b99a2f7159b2225f374cd378d71302fa28414ac")
	pkc_adr := Address("26bnyi8VtSMREjx6Pntsppgrwb9YZRzMw78")
	pkc_type, pkc_arr := ClassifyScript(pkc_scr, &DogeMainNetChain)
	if pkc_type != ScriptTypeP2PK {
		t.Errorf("Wrong script type: %v vs %v", pkc_type, ScriptTypeP2PK)
	}
	if len(pkc_arr) != 1 || pkc_arr[0] != pkc_adr {
		t.Errorf("Wrong address: %v vs %v", pkc_arr[0], pkc_adr)
	}
	// NullData
	nud_scr := hx2b("6a1454f6fb64f14b756d118a96a57a2f9ebf4b4708fe")
	nud_type, nud_arr := ClassifyScript(nud_scr, &DogeMainNetChain)
	if nud_type != ScriptTypeNullData {
		t.Errorf("Wrong script type: %v vs %v", nud_type, ScriptTypeNullData)
	}
	if len(nud_arr) != 0 {
		t.Errorf("Wrong address: expecting zero")
	}
	// MultiSig
	ms_scr := hx2b("514104cc71eb30d653c0c3163990c47b976f3fb3f37cccdcbedb169a1dfef58bbfbfaff7d8a473e7e2e6d317b87bafe8bde97e3cf8f065dec022b51d11fcdd0d348ac4410461cbdcc5409fb4b4d42b51d33381354d80e550078cb532a34bfa2fcfdeb7d76519aecc62770f5b0e4ef8551946d8a540911abe3e7854a26f39f58b25c15342af52ae")
	ms_adrs := []Address{"26jcHKT3WNLEbV21sg2L1jiHwERzq1EhyLq", "26xZbxtkQTJa3K4a1fooffmgTJUuSYnDtBa"}
	ms_type, ms_arr := ClassifyScript(ms_scr, &DogeMainNetChain)
	if ms_type != ScriptTypeMultiSig {
		t.Errorf("Wrong script type: %v vs %v", ms_type, ScriptTypeMultiSig)
	}
	if len(ms_arr) != 2 || ms_arr[0] != ms_adrs[0] || ms_arr[1] != ms_adrs[1] {
		t.Errorf("Wrong addresses: %v vs %v", ms_arr, ms_adrs)
	}
}
