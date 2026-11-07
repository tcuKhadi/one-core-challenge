package models

type Post struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Content   string `json:"content"`
	Likes     int64  `json:"likes"`
	Comments  int64  `json:"comments"`
	CreatedAt string `json:"created_at"`
}

type Comment struct {
	PostID    int64  `json:"post_id"`
	Username  string `json:"username"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type ContentReq struct {
	Content string `json:"content"`
}

type UserProfile struct {
	Username      string `json:"username"`
	PostCount     int64  `json:"post_count"`
	ProfileVisits int64  `json:"profile_visits"`
}
