package database

import "database/sql"

type Video struct {
	Id         string `db:"id" json:"id"`
	MagnetLink string `db:"magnet_link" json:"magnet_link"`
	FilePath   string `db:"file_path" json:"file_path"`
	CreatedAt  int64  `db:"created_at" json:"created_at"`
	Deleted    bool   `db:"deleted" json:"deleted"`
}

func (s *service) CreateVideo(video Video) error {
	stmt := `
		INSERT OR IGNORE INTO videos (
			id,
			magnet_link,
			file_path,
			created_at
		) VALUES (?, ?, ?, ?)
	`

	_, err := s.db.Exec(stmt,
		video.Id,
		video.MagnetLink,
		video.FilePath,
		video.CreatedAt,
	)

	return err
}

func (s *service) GetVideo(videoId string) (Video, error) {
	var video Video

	stmt := `
		SELECT 
			id, 
			magnet_link,
			file_path,
			created_at, 
			deleted
		FROM videos
		WHERE id = ?
	`

	row := s.db.QueryRow(stmt, videoId)

	err := row.Scan(&video.Id, &video.MagnetLink, &video.FilePath, &video.CreatedAt, &video.Deleted)

	return video, err
}

func (s *service) GetAllVideos() ([]Video, error) {
	stmt := `
		SELECT 
			id, 
			magnet_link, 
			file_path, 
			created_at, 
			deleted
		FROM videos
		WHERE deleted = FALSE
		ORDER BY created_at DESC
	`

	rows, err := s.db.Query(stmt)
	if err != nil {
		if err == sql.ErrNoRows {
			return []Video{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	videos := []Video{}

	for rows.Next() {
		var v Video
		err := rows.Scan(&v.Id, &v.MagnetLink, &v.FilePath, &v.CreatedAt, &v.Deleted)
		if err != nil {
			return nil, err
		}
		videos = append(videos, v)
	}

	// handle possible iteration error
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return videos, nil
}

func (s *service) DeleteVideo(videoId string) error {
	stmt := `DELETE FROM 
		videos 
		WHERE id = ?`

	if _, err := s.db.Exec(stmt, videoId); err != nil {
		return err
	}

	return nil
}
