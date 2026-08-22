package adapter

type Article struct {
	ID      int
	URL     string
	Title   string
	Author  string
	Content string
}

type Adapter interface {
	Name() string
	Feeds() ([]string, error)
	Articles(feed string) ([]Article, error)
	Download(id int) ([]byte, error)
	Ping() error
}
