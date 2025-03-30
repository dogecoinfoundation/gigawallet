package main

/* THIS IS A MESS :)
 * When it's vaguely working it will need a refactor
 * into several files but for now, enjoy the spaghetti
 */

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

type InvResp struct {
	Items []giga.PublicInvoice
}

func (b *BrowserAccount) updateInvoices(c giga.Config) {
	url := adminURL(c, fmt.Sprintf("account/%s/invoices", b.ID))
	s, err := getURL(url)
	if err != nil {
		log.Fatalf("failed to get blasdfhjafdk", err)
	}
	resp := InvResp{}
	if err := json.NewDecoder(strings.NewReader(s)).Decode(&resp); err != nil {
		log.Fatalf("bad bad", s, err)
		// bad
	}
	b.Invoices = resp.Items
}

type Browser struct {
	newAccountField *tview.InputField
	accountList     *tview.List
	invoiceList     *tview.Table
	pages           *tview.Pages
	state           *BrowserState
	config          giga.Config
}

func LaunchBrowser(conf giga.Config) {

	app := tview.NewApplication().EnableMouse(true)
	s := BrowserState{
		Accounts: map[string]*BrowserAccount{},
	}
	b := Browser{
		state:  &s,
		pages:  tview.NewPages(),
		config: conf,
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
	data := []string{"crumpets"}
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

	accUI, input, list := b.buildAccountList()
	b.accountList = list
	b.newAccountField = input

	invUI, invList := b.buildInvoiceList()
	b.invoiceList = invList

	grid := tview.NewGrid().
		SetRows(1, 0).
		SetColumns(30, 0).
		SetBorders(true).
		AddItem(newPrimitive("🐶 GigaWallet Browser"), 0, 0, 1, 2, 0, 0, false).
		AddItem(accUI, 1, 0, 1, 1, 0, 15, true).
		AddItem(invUI, 1, 1, 1, 1, 0, 0, false)

	b.pages.AddPage("main", grid, true, true)
}

func (b *Browser) switchAccount(a *BrowserAccount) {
	b.state.Current = a
	a.updateInvoices(b.config)
	b.refreshUI()
}

func (b *Browser) refreshUI() {
	b.updateAccountList()
	b.updateInvoiceTable()
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

func (b *Browser) updateInvoiceTable() {
	t := b.invoiceList

	t.Clear()
	// draw headers
	t.
		SetCellSimple(0, 0, "ID").
		SetCellSimple(0, 1, "Created").
		SetCellSimple(0, 2, "Amount").
		SetCellSimple(0, 3, "Detected").
		SetCellSimple(0, 4, "Confirmed")
	for i, inv := range b.state.Current.Invoices {
		i++
		t.SetCellSimple(i, 0, truncateAddr(inv.ID))
		t.SetCellSimple(i, 1, inv.Created.Format("02 Jan 06 15:04 MST"))
		t.SetCellSimple(i, 2, inv.Total.String())
		if inv.TotalDetected {
			t.SetCell(i, 3,
				tview.NewTableCell("✔").
					SetTextColor(tcell.ColorGreen).
					SetAlign(tview.AlignCenter))
		} else {
			t.SetCell(i, 3,
				tview.NewTableCell("✘").
					SetTextColor(tcell.ColorRed).
					SetAlign(tview.AlignCenter))

		}
		if inv.TotalConfirmed {
			t.SetCell(i, 4,
				tview.NewTableCell("✔").
					SetTextColor(tcell.ColorGreen).
					SetAlign(tview.AlignCenter))

		} else {
			t.SetCell(i, 4,
				tview.NewTableCell("✘").
					SetTextColor(tcell.ColorRed).
					SetAlign(tview.AlignCenter))

		}
	}
}

func (b *Browser) buildInvoiceList() (*tview.Grid, *tview.Table) {
	table := tview.NewTable().
		SetBorders(false).
		SetSeparator(rune('|')).
		SetFixed(1, 0).
		SetSelectable(true, false).
		SetCellSimple(0, 0, "hello")

	grid := tview.NewGrid().
		SetRows(0).
		SetColumns(0).
		SetBorders(false).
		AddItem(table, 0, 0, 1, 1, 0, 0, true)

	return grid, table
}
func (b *Browser) buildAccountList() (*tview.Grid, *tview.InputField, *tview.List) {
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

func truncateAddr(a giga.Address) string {
	s := string(a)
	return fmt.Sprintf("%s...%s", s[0:5], s[len(s)-5:])
}
