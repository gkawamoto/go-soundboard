package ui

import (
	"path/filepath"
	"sync"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var ValidSoundKeys = []rune{
	'`', '1', '2', '3', '4', '5', '6', '7', '8', '9', '0', '-', '=',
	'q', 'w', 'e', 'r', 't', 'y', 'u', 'i', 'o', 'p', '[', ']', '\\',
	'~', '!', '@', '#', '$', '%', '^', '&', '*', '(', ')', '_', '+',
	'Q', 'W', 'E', 'R', 'T', 'Y', 'U', 'I', 'O', 'P', '{', '}', '|',
}

type controller interface {
	PlaySound(sound string) error
	DownloadFile(target string) (string, error)
	Save() error
	Load() error
	FileSearch(text string) ([]string, error)
	SearchAPI(text string) ([]string, error)
	AddFile(file string, key rune)
	DeleteItem(index int, key rune)
	StopAllSounds()
}

func NewApp(c controller) *App {
	result := &App{controller: c, statusLock: &sync.Mutex{}}
	result.render()
	return result
}

func (a *App) render() {
	main := tview.NewFlex()
	main.SetDirection(tview.FlexRow).SetBackgroundColor(tcell.ColorBlack)
	a.app = tview.NewApplication().SetRoot(main, true)

	title := tview.NewTextView()
	title.SetText("Go Soundboard")
	title.SetBackgroundColor(tcell.ColorDarkRed)
	main.AddItem(title, 1, 1, true)

	top := tview.NewFlex()
	top.SetDirection(tview.FlexColumn)
	main.AddItem(top, 0, 1, true)

	leftPanels := tview.NewFlex()
	leftPanels.SetDirection(tview.FlexRow)
	top.AddItem(leftPanels, 0, 1, true)

	a.soundList = tview.NewList()
	a.soundList.ShowSecondaryText(false).SetTitle(" sound list ").SetBorder(true)
	leftPanels.AddItem(a.soundList, 0, 1, true)
	a.app.SetFocus(a.soundList)

	rightPanels := tview.NewFlex()
	rightPanels.SetDirection(tview.FlexRow)
	top.AddItem(rightPanels, 0, 1, true)

	addFileForm := tview.NewForm()
	addFileForm.SetTitle(" add file ").SetBorder(true)
	rightPanels.AddItem(addFileForm, 9, 1, true)

	a.filenameFormInput = tview.NewInputField()
	a.filenameFormInput.SetLabel("filename")
	a.filenameFormInput.SetAutocompleteFunc(func(currentText string) (entries []string) {
		data, err := a.controller.FileSearch(currentText)
		if err != nil {
			a.Error(err.Error())
			return nil
		}
		return data
	})
	addFileForm.AddFormItem(a.filenameFormInput)

	a.keyFormInput = tview.NewInputField()
	a.keyFormInput.SetLabel("     key")
	a.keyFormInput.SetFieldWidth(5)
	a.keyFormInput.SetAcceptanceFunc(func(textToCheck string, lastChar rune) bool {
		if utf8.RuneCountInString(textToCheck) > 1 {
			return false
		}
		for _, k := range ValidSoundKeys {
			if k == lastChar {
				return true
			}
		}
		return false
	})
	addFileForm.AddFormItem(a.keyFormInput)
	addFileForm.AddButton("add", func() {
		if a.keyFormInput.GetText() == "" {
			addFileForm.SetFocus(1)
			return
		}
		if a.filenameFormInput.GetText() == "" {
			addFileForm.SetFocus(0)
			return
		}
		a.controller.AddFile(a.filenameFormInput.GetText(), rune(a.keyFormInput.GetText()[0]))
		a.resetForms()
		a.app.SetFocus(a.soundList)
	})

	searchMyInstantsForm := tview.NewForm()
	searchMyInstantsForm.SetTitle(" search myinstants ").SetBorder(true)
	rightPanels.AddItem(searchMyInstantsForm, 7, 1, true)

	a.searchFormInput = tview.NewInputField()
	a.searchFormInput.SetLabel("  search")
	searchMyInstantsForm.AddFormItem(a.searchFormInput)
	a.searchFormInput.SetAutocompleteFunc(func(currentText string) (entries []string) {
		data, err := a.controller.SearchAPI(currentText)
		if err != nil {
			a.Error(err.Error())
			return nil
		}
		return data
	})

	searchMyInstantsForm.AddButton("play", func() {
		go a.playSound(a.searchFormInput.GetText())
	})
	searchMyInstantsForm.AddButton("download", func() {
		file := a.searchFormInput.GetText()
		go func() {
			a.statusLock.Lock()
			index := a.getStatusReference()

			a.statusList.InsertItem(0, "⏬ downloading "+file, "", 0, func() {})
			a.statusList.SetCurrentItem(0)
			a.statusLock.Unlock()

			path, err := a.controller.DownloadFile(file)
			if err != nil {
				a.Error(err.Error())
				return
			}

			a.resetForms()
			a.filenameFormInput.SetText(path)
			addFileForm.SetFocus(0)
			a.app.SetFocus(addFileForm)

			a.statusLock.Lock()
			index = a.statusList.GetItemCount() - index
			a.statusList.SetItemText(index, "👍 downloaded "+filepath.Base(file), "")
			a.statusLock.Unlock()
			a.app.Draw()
		}()
	})

	rightPanels.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			a.resetForms()
			a.app.SetFocus(a.soundList)
			return event
		}
		return event
	})

	a.statusList = tview.NewList()
	a.statusList.ShowSecondaryText(false).SetBorder(true)
	rightPanels.AddItem(a.statusList, 0, 1, true)

	bottom := tview.NewTextView()
	bottom.SetText("commands: [s]ave | [l]oad | add [f]ile | [d]elete sound | search [m]y instants | stop [a]ll sounds")
	main.AddItem(bottom, 1, 1, true)

	a.soundList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 's':
			go a.save()
		case 'l':
			err := a.controller.Load()
			if err != nil {
				a.Error(err.Error())
			}
		case 'f':
			a.app.SetFocus(addFileForm)
		case 'm':
			searchMyInstantsForm.SetFocus(0)
			a.app.SetFocus(searchMyInstantsForm)
		case 'd':
			index := a.soundList.GetCurrentItem()
			_, secondary := a.soundList.GetItemText(index)
			a.controller.DeleteItem(index, rune(secondary[0]))
		case 'a':
			a.controller.StopAllSounds()
		}
		return event
	})
}

type App struct {
	controller controller
	app        *tview.Application

	soundList  *tview.List
	statusList *tview.List

	filenameFormInput *tview.InputField
	keyFormInput      *tview.InputField
	searchFormInput   *tview.InputField

	soundCount int
	statusLock *sync.Mutex
}

func (a *App) ClearSoundList() {
	a.soundList.Clear()
}

func (a *App) resetForms() {
	a.filenameFormInput.SetText("")
	a.filenameFormInput.Autocomplete()

	a.keyFormInput.SetText("")

	a.searchFormInput.SetText("")
	a.searchFormInput.Autocomplete()
}

func (a *App) getStatusReference() int {
	a.soundCount++
	id := a.soundCount

	return id
}

func (a *App) AddSound(file string, key rune) {
	a.soundList.AddItem(file, string(key), key, func() {
		go a.playSound(file)
	})
}

func (a *App) save() {
	a.statusLock.Lock()
	index := a.getStatusReference()

	a.statusList.InsertItem(0, "🧐 saving data.json...", "", 0, func() {})
	a.statusList.SetCurrentItem(0)
	a.statusLock.Unlock()

	a.app.Draw()

	err := a.controller.Save()

	a.statusLock.Lock()
	index = a.statusList.GetItemCount() - index

	if err != nil {
		a.statusList.SetItemText(index, "😔 could not save: "+err.Error(), "")
	} else {
		a.statusList.SetItemText(index, "😄 saved data.json!", "")
	}

	a.statusLock.Unlock()
	a.app.Draw()
}

func (a *App) playSound(file string) {
	a.statusLock.Lock()
	index := a.getStatusReference()

	a.statusList.InsertItem(0, "🔈 "+file, "", 0, func() {})
	a.statusList.SetCurrentItem(0)
	a.statusLock.Unlock()

	err := a.controller.PlaySound(file)
	if err != nil {
		a.Error(err.Error())
	}

	a.statusLock.Lock()
	index = a.statusList.GetItemCount() - index
	a.statusList.SetItemText(index, "🔇 "+file, "")
	a.statusLock.Unlock()
	a.app.Draw()
}

func (a *App) Info(text string) {
	a.statusList.AddItem("👉 "+text, "", 0, func() {})
	a.app.Draw()
}

func (a *App) Error(text string) {
	a.statusList.AddItem("🔴 "+text, "", 0, func() {})
	a.app.Draw()
}

func (a *App) SetSelectedSound(index int) {
	a.soundList.SetCurrentItem(index)
}

func (a *App) Run() error {
	return a.app.Run()
}
