package models

import "time"

const (
	FormatPDF  = "pdf"
	FormatEPUB = "epub"
)

type Location struct {
	Page     int       `bson:"page" json:"page"`
	Pages    int       `bson:"pages" json:"pages"`
	Cfi      string    `bson:"cfi" json:"cfi"`
	Offset   float64   `bson:"offset" json:"offset"`
	Percent  float64   `bson:"percent" json:"percent"`
	Label    string    `bson:"label" json:"label"`
	Recorded time.Time `bson:"recorded" json:"recorded"`
}

type Book struct {
	Key         string     `bson:"key" json:"key"`
	Owner       string     `bson:"owner" json:"owner"`
	Title       string     `bson:"title" json:"title"`
	Author      string     `bson:"author" json:"author"`
	Format      string     `bson:"format" json:"format"`
	SizeBytes   int64      `bson:"sizeBytes" json:"sizeBytes"`
	Current     Location   `bson:"current" json:"current"`
	History     []Location `bson:"history" json:"history"`
	Finished    bool       `bson:"finished" json:"finished"`
	OpenCount   int        `bson:"openCount" json:"openCount"`
	SecondsRead int64      `bson:"secondsRead" json:"secondsRead"`
	CreatedAt   time.Time  `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time  `bson:"updatedAt" json:"updatedAt"`
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
