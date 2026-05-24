package provider

type MediaType string
type Status string
type Season string

const (
	MediaTypeAnime  MediaType = "anime"
	MediaTypeMovie  MediaType = "movie"
	MediaTypeSeries MediaType = "series"
)

const (
	StatusReleasing   Status = "RELEASING"
	StatusFinished    Status = "FINISHED"
	StatusCancelled   Status = "CANCELLED"
	StatusNotYetAired Status = "NOT_YET_AIRED"
)

const (
	SeasonFall   Season = "FALL"
	SeasonSpring Season = "SPRING"
	SeasonWinter Season = "WINTER"
	SeasonSummer Season = "SUMMER"
)

type Media struct {
	ID            string    `json:"id"`
	Type          MediaType `json:"type"`
	Title         string    `json:"title"`
	OriginalTitle string    `json:"original_title"`
	Cover         string    `json:"cover"`
	Banner        string    `json:"banner"`
	Description   string    `json:"description"`
	Score         float64   `json:"score"`
	Genres        []string  `json:"genres"`
	Status        Status    `json:"status"`
	Season        Season    `json:"season"`
	SeasonYear    int       `json:"season_year"`
	TotalEpisodes *int      `json:"total_episodes"`
	Duration      *int      `json:"duration"`
	NextAiring    *Airing   `json:"next_airing"`
}

type Airing struct {
	Episode  int   `json:"episode"`
	AiringAt int64 `json:"airing_at"`
}

type Episode struct {
	Id       int    `json:"id"`
	Number   int    `json:"number"`
	Title    string `json:"title"`
	AirDate  string `json:"air_date"`
	Overview string `json:"overview"`
	Image    string `json:"image"`
}

type EpisodeList struct {
	Episodes []Episode `json:"episodes"`
	Specials []Episode `json:"specials"`
}

type Source struct {
	Title     string `json:"title"`
	MagnetURI string `json:"magnet_uri"`
	Seeders   int    `json:"seeders"`
	Leechers  int    `json:"leechers"`
	Size      int64  `json:"size"`
	InfoHash  string `json:"info_hash"`
}
