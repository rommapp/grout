package romm

type ScreenScrapper struct {
	BezelURL           string   `json:"bezel_url"`
	Box2DURL           string   `json:"box2d_url"`
	Box2DSideURL       string   `json:"box2d_side_url"`
	Box2DBackURL       string   `json:"box2d_back_url"`
	Box3DURL           string   `json:"box3d_url"`
	FanartURL          string   `json:"fanart_url"`
	FullboxURL         string   `json:"fullbox_url"`
	LogoURL            string   `json:"logo_url"`
	ManualURL          string   `json:"manual_url"`
	MarqueeURL         string   `json:"marquee_url"`
	MiximageURL        string   `json:"miximage_url"`
	PhysicalURL        string   `json:"physical_url"`
	ScreenshotURL      string   `json:"screenshot_url"`
	SteamgridURL       string   `json:"steamgrid_url"`
	TitleScreenURL     string   `json:"title_screen_url"`
	VideoURL           string   `json:"video_url"`
	VideoNormalizedURL string   `json:"video_normalized_url"`
	BezelPath          string   `json:"bezel_path"`
	Box2DBackPath      string   `json:"box2d_back_path"`
	Box3DPath          string   `json:"box3d_path"`
	FanartPath         string   `json:"fanart_path"`
	MiximagePath       string   `json:"miximage_path"`
	PhysicalPath       string   `json:"physical_path"`
	MarqueePath        string   `json:"marquee_path"`
	LogoPath           string   `json:"logo_path"`
	VideoPath          string   `json:"video_path"`
	SsScore            string   `json:"ss_score"`
	FirstReleaseDate   int      `json:"first_release_date"`
	AlternativeNames   []string `json:"alternative_names"`
	Companies          []string `json:"companies"`
	Franchises         []string `json:"franchises"`
	GameModes          []string `json:"game_modes"`
	Genres             []string `json:"genres"`
	PlayerCount        string   `json:"player_count"`
}
