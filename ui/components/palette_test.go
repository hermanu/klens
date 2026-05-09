package components_test

import (
	"testing"

	"github.com/manu/klens/ui/components"
)

func TestPalette_Filter(t *testing.T) {
	p := components.NewPalette(nil) // nil = use default resource commands
	p = p.SetInput("sec")

	filtered := p.Filtered()
	if len(filtered) != 1 {
		t.Fatalf("want 1 result for 'sec', got %d: %v", len(filtered), filtered)
	}
	if filtered[0].Name != "secrets" {
		t.Errorf("want secrets, got %s", filtered[0].Name)
	}
}

func TestPalette_FilterEmpty(t *testing.T) {
	p := components.NewPalette(nil)
	p = p.SetInput("")
	if len(p.Filtered()) == 0 {
		t.Error("want all commands when input is empty")
	}
}

func TestPalette_Selected(t *testing.T) {
	p := components.NewPalette(nil)
	sel := p.Selected()
	if sel == nil {
		t.Fatal("want non-nil Selected() on fresh palette")
	}
}

func TestPalette_Navigation(t *testing.T) {
	cmds := []components.PaletteCmd{
		{Name: "pods",    Desc: "list pods",    Alias: ":po"},
		{Name: "secrets", Desc: "list secrets", Alias: ":sec"},
	}
	p := components.NewPalette(cmds)
	// starts at index 0
	if p.Selected().Name != "pods" {
		t.Errorf("want pods, got %s", p.Selected().Name)
	}
	// move down
	p, _ = p.MoveDown()
	if p.Selected().Name != "secrets" {
		t.Errorf("want secrets after MoveDown, got %s", p.Selected().Name)
	}
	// move up
	p, _ = p.MoveUp()
	if p.Selected().Name != "pods" {
		t.Errorf("want pods after MoveUp, got %s", p.Selected().Name)
	}
}
