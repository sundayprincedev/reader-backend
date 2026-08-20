package models

import (
	"encoding/json"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	FormatPDF  = "pdf"
	FormatEPUB = "epub"
)

type Viewport struct {
	Width  int     `bson:"width" json:"width"`
	Height int     `bson:"height" json:"height"`
	Scale  float64 `bson:"scale" json:"scale"`
}

type Location struct {
	Page     int       `bson:"page" json:"page"`
	Pages    int       `bson:"pages" json:"pages"`
	Cfi      string    `bson:"cfi" json:"cfi"`
	Offset   float64   `bson:"offset" json:"offset"`
	Percent  float64   `bson:"percent" json:"percent"`
	Label    string    `bson:"label" json:"label"`
	Viewport Viewport  `bson:"viewport" json:"viewport"`
	Manual   bool      `bson:"manual" json:"manual"`
	Recorded time.Time `bson:"recorded" json:"recorded"`
}

type Book struct {
	Key         string         `bson:"key" json:"key"`
	Title       string         `bson:"title" json:"title"`
	Author      string         `bson:"author" json:"author"`
	Format      string         `bson:"format" json:"format"`
	SizeBytes   int64          `bson:"sizeBytes" json:"sizeBytes"`
	FileID      *bson.ObjectID `bson:"fileId,omitempty" json:"-"`
	HasFile     bool           `bson:"hasFile" json:"hasFile"`
	Current     Location       `bson:"current" json:"current"`
	History     []Location     `bson:"history" json:"history"`
	Finished    bool           `bson:"finished" json:"finished"`
	OpenCount   int            `bson:"openCount" json:"openCount"`
	SecondsRead int64          `bson:"secondsRead" json:"secondsRead"`
	CreatedAt   time.Time      `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time      `bson:"updatedAt" json:"updatedAt"`
}

func (b Book) MarshalJSON() ([]byte, error) {
	type plain Book

	return json.Marshal(struct {
		plain
		Started   bool `json:"started"`
		Removable bool `json:"removable"`
	}{plain(b), b.Started(), b.Removable()})
}

func (b Book) Started() bool {
	return b.Current.Percent > 0 || len(b.History) > 0
}

func (b Book) Removable() bool {
	return b.Finished || !b.Started()
}

type RegisterRequest struct {
	Key       string `json:"key"`
	Title     string `json:"title"`
	Author    string `json:"author"`
	Format    string `json:"format"`
	SizeBytes int64  `json:"sizeBytes"`
}

type ProgressRequest struct {
	Page        int     `json:"page"`
	Pages       int     `json:"pages"`
	Cfi         string  `json:"cfi"`
	Offset      float64 `json:"offset"`
	Percent     float64 `json:"percent"`
	Label       string  `json:"label"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	Scale       float64 `json:"scale"`
	Manual      bool    `json:"manual"`
	SecondsRead int64   `json:"secondsRead"`
}

type LibraryStats struct {
	Books       int     `json:"books"`
	Started     int     `json:"started"`
	Finished    int     `json:"finished"`
	SecondsRead int64   `json:"secondsRead"`
	AveragePct  float64 `json:"averagePercent"`
}

type LibraryResponse struct {
	Books []Book       `json:"books"`
	Stats LibraryStats `json:"stats"`
}
