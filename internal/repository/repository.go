package repository

type Repository interface {
	Save(id, url string) error
	Get(id string) (string, bool)
	FindByURL(url string) (string, bool)
}
