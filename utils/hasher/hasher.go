package hasher

import "golang.org/x/crypto/bcrypt"

type Hasher struct {
	cost int
}

func New(cfg Config) *Hasher {
	return &Hasher{cost: cfg.Cost}
}

func (h *Hasher) Hash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (h *Hasher) Check(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
