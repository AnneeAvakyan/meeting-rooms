package services

type TokenGenerator interface {
	GenerateToken(userID int64) (string, error)
}
