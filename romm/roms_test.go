package romm

import "testing"

func TestCanonicalLocalBasename(t *testing.T) {
	tests := []struct {
		name string
		rom  Rom
		want string
	}{
		{
			name: "simple single file uses the file basename",
			rom: Rom{
				HasMultipleFiles: false,
				FsName:           "Metroid_ Scrolls 6 (1.1).gba",
				FsNameNoExt:      "Metroid_ Scrolls 6 (1.1)",
				Files:            []RomFile{{FileName: "Metroid_ Scrolls 6 (1.1).gba"}},
			},
			want: "Metroid_ Scrolls 6 (1.1)",
		},
		{
			// The issue #242 case: RomM stores the ROM as a folder (fs_name is the
			// clean folder name) containing one tagged file. The save and the
			// downloaded ROM on disk are named after the *file*, not the folder.
			name: "nested single file uses the inner file basename not the folder",
			rom: Rom{
				HasMultipleFiles: false,
				FsName:           "Kingdom Hearts - Chain of Memories",
				FsNameNoExt:      "Kingdom Hearts - Chain of Memories",
				Files:            []RomFile{{FileName: "Kingdom Hearts - Chain of Memories (Europe) (En,Fr,De,Es,It).gba"}},
			},
			want: "Kingdom Hearts - Chain of Memories (Europe) (En,Fr,De,Es,It)",
		},
		{
			name: "multi file uses the m3u basename (fs_name_no_ext)",
			rom: Rom{
				HasMultipleFiles: true,
				FsName:           "Final Fantasy VII",
				FsNameNoExt:      "Final Fantasy VII",
				Files: []RomFile{
					{FileName: "Final Fantasy VII (Disc 1).bin"},
					{FileName: "Final Fantasy VII (Disc 2).bin"},
				},
			},
			want: "Final Fantasy VII",
		},
		{
			name: "no file metadata falls back to fs_name_no_ext",
			rom: Rom{
				HasMultipleFiles: false,
				FsNameNoExt:      "Some Game (USA)",
				Files:            nil,
			},
			want: "Some Game (USA)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rom.CanonicalLocalBasename(); got != tt.want {
				t.Errorf("CanonicalLocalBasename() = %q, want %q", got, tt.want)
			}
		})
	}
}
