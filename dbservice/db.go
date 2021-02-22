package dbservice

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

var initialNGWords = []string{"戌神ころね", "リゼ・ヘルエスタ", "Vtuber", "VTuber", "vtuber", "バーチャルユーチューバー", "バーチャルYouTuber", "笹木咲", "戌亥とこ"}

type DbService struct {
	db *sql.DB
}

func NewDbService(path string) (*DbService, error) {
	dbService := &DbService{}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return dbService, err
	}
	dbService.db = db
	return dbService, nil
}

func (dbService *DbService) Init() error {
	_, err := dbService.db.Exec(
		`CREATE TABLE IF NOT EXISTS ng_words (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			word VARCHAR(100),
			UNIQUE(word)
		)`,
	)
	if err != nil {
		return err
	}

	// 初期値を入力
	for _, word := range initialNGWords {
		_, err := dbService.db.Exec(
			`INSERT INTO ng_words (word) VALUES (?) ON CONFLICT(word) DO NOTHING`,
			word,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (dbService *DbService) InsertNg(word string) (int64, error) {
	res, err := dbService.db.Exec(
		`INSERT INTO ng_words (word) VALUES (?)`,
		word,
	)
	id, err := res.LastInsertId()
	if err != nil {
		return -1, err
	}
	return id, err
}

func (dbService *DbService) SelectAllNgs() ([]string, error) {
	var words []string
	var word string
	res, err := dbService.db.Query(
		`SELECT word FROM ng_words`,
	)

	for res.Next() {
		err = res.Scan(&word)
		if err != nil {
			return nil, err
		}
		words = append(words, word)
	}
	return words, nil
}

func (dbService *DbService) DeleteNg(word string) error {
	_, err := dbService.db.Exec(
		`DELETE FROM ng_words WHERE word = ?`,
		word,
	)
	return err
}

func (dbService *DbService) FindByWord(word string) (string, error) {
	result := ""
	row := dbService.db.QueryRow(
		`DELETE FROM ng_words WHERE word = ?`,
		word,
	)
	err := row.Scan(&result)
	return result, err
}
