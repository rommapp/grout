package muos

import "testing"

func TestMuosLayoutWithM3U(t *testing.T) {
	folder, playlist, beside := HiddenLayout("Game", []byte("a.chd\nb.chd\n"))
	if folder != "_Game" || !beside {
		t.Fatalf("folder %q beside %v", folder, beside)
	}
	if string(playlist) != "_Game/a.chd\n_Game/b.chd\n" {
		t.Fatalf("playlist %q", playlist)
	}
}

func TestMuosLayoutKeepsCommentsAndCRLF(t *testing.T) {
	_, playlist, _ := HiddenLayout("Game", []byte("#EXTM3U\r\na.chd\r\n\r\nb.chd"))
	if string(playlist) != "#EXTM3U\n_Game/a.chd\n\n_Game/b.chd" {
		t.Fatalf("playlist %q", playlist)
	}
}

func TestMuosLayoutWithoutM3U(t *testing.T) {
	folder, playlist, beside := HiddenLayout("Game", nil)
	if folder != "_Game" || !beside || playlist != nil {
		t.Fatalf("folder %q playlist %q beside %v", folder, playlist, beside)
	}
}
