package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"slices"
	"strings"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type BrowserState struct {
	Accounts map[string]*BrowserAccount
	Current  *BrowserAccount
}

type BrowserAccount struct {
	ID       string
	Invoices []giga.PublicInvoice
}

func (b BrowserAccount) updateInvoices(c giga.Config) {
	url := adminURL(c, fmt.Sprintf("account/%s/invoices", b.ID))
	s, err := getURL(url)
	if err != nil {
		log.Fatalf("failed to get blasdfhjafdk", err)
	}
	if err := json.NewDecoder(strings.NewReader(s)).Decode(&b.Invoices); err != nil {
		log.Fatalf("bad bad", err)
		// bad
	}
}

type Browser struct {
	newAccountField *tview.InputField
	accountList     *tview.List
	pages           *tview.Pages
	state           *BrowserState
}

func LaunchBrowser(conf giga.Config) {

	app := tview.NewApplication().EnableMouse(true)
	s := BrowserState{
		Accounts: map[string]*BrowserAccount{},
	}
	b := Browser{
		state: &s,
		pages: tview.NewPages(),
	}
	b.loadState()
	b.buildMainView()
	b.refreshUI()
	if err := app.SetRoot(b.pages, true).SetFocus(b.pages).Run(); err != nil {
		panic(err)
	}
}

func (b *Browser) loadState() {
	// TODO: load from .. something?
	data := []string{"crumpets", "raffe", "tjs", "inevitable", "bluezr", "michi", "ed", "Marshall", "Jens"}
	for _, n := range data {
		b.state.Accounts[n] = &BrowserAccount{ID: n}
	}
	b.state.Current = b.state.Accounts[data[0]]
}

func (b *Browser) buildMainView() {
	newPrimitive := func(text string) tview.Primitive {
		return tview.NewTextView().
			SetTextAlign(tview.AlignCenter).
			SetText(text)
	}

	accUI, input, list := b.buildAccountList(b.state)
	b.accountList = list
	b.newAccountField = input

	grid := tview.NewGrid().
		SetRows(1, 0).
		SetColumns(30, 0).
		SetBorders(true).
		AddItem(newPrimitive("🐶 GigaWallet Browser"), 0, 0, 1, 2, 0, 0, false).
		AddItem(accUI, 1, 0, 1, 1, 0, 15, true).
		AddItem(newPrimitive("Invoices"), 1, 1, 1, 1, 0, 0, false)

	b.pages.AddPage("main", grid, true, true)
}

func (b *Browser) switchAccount(a *BrowserAccount) {
	b.state.Current = a
	b.refreshUI()
}

func (b *Browser) refreshUI() {
	b.updateAccountList()
}

func (b *Browser) updateAccountList() {
	b.accountList.Clear()
	accounts := []*BrowserAccount{}
	for _, acc := range b.state.Accounts {
		accounts = append(accounts, acc)
	}
	slices.SortFunc(accounts, func(a, b *BrowserAccount) int {
		return cmp.Compare(a.ID, b.ID)
	})

	selected := 0
	for i, acc := range accounts {
		name := fmt.Sprintf("  %s", acc.ID)
		if acc.ID == b.state.Current.ID {
			selected = i
			name = fmt.Sprintf("> %s", acc.ID)
		}
		accCopy := acc // for closure, because Go is weird
		b.accountList.AddItem(name, "", rune(0), func() {
			b.switchAccount(accCopy)
		})
	}
	b.accountList.SetCurrentItem(selected)
}

func (b *Browser) buildAccountList(s *BrowserState) (*tview.Grid, *tview.InputField, *tview.List) {
	newAccountField := tview.NewInputField().
		SetLabel("Find Account: ").
		SetFieldWidth(15).
		SetDoneFunc(func(key tcell.Key) {
			// save a thing
		})
	list := tview.NewList().
		SetHighlightFullLine(true).
		ShowSecondaryText(false)

	grid := tview.NewGrid().
		SetRows(1, 0).
		SetColumns(0).
		SetBorders(false).
		AddItem(newAccountField, 0, 0, 1, 1, 0, 0, true).
		AddItem(list, 1, 0, 1, 1, 0, 0, false)

	return grid, newAccountField, list
}

func adminURL(c giga.Config, path string) string {
	host := c.WebAPI.AdminBind
	if host == "" {
		host = "localhost"
	}
	base := fmt.Sprintf("http://%s:%s/", host, c.WebAPI.AdminPort)
	u, _ := url.Parse(base)
	p, _ := url.Parse(path)
	return u.ResolveReference(p).String()
}
