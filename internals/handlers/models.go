package handlers

import (
	"forum/internals/forum"
)

// PageData represents the common data passed to templates.
type PageData struct {
	User        *User
	Posts       []forum.Post
	Categories  []Category
	Query       string
	CurrentPage int
	TotalPages  int
	CategoryID  int
	MyPosts     bool
	MyLiked     bool
	MyComments  bool
	HasPrev     bool
	HasNext     bool
	PrevPage    int
	NextPage    int
}

type User struct {
	ID       int
	Username string
}

type Category struct {
	ID   int
	Name string
}

// PostDetailData is the data passed to the post details template.
type PostDetailData struct {
	User       *User
	Post       *forum.Post
	Categories []Category
}
