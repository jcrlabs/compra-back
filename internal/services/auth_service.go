package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jcrlabs/compra-back/internal/models"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	jwt.RegisteredClaims
}

type userRepo interface {
	FindByEmail(email string) (*models.User, error)
	FindByID(id uuid.UUID) (*models.User, error)
	Create(u *models.User) error
	Update(u *models.User) error
}

type AuthService struct {
	users     userRepo
	jwtSecret []byte
	ttl       time.Duration
}

func NewAuthService(users userRepo, jwtSecret string, ttlHours int) *AuthService {
	return &AuthService{
		users:     users,
		jwtSecret: []byte(jwtSecret),
		ttl:       time.Duration(ttlHours) * time.Hour,
	}
}

func (s *AuthService) HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(b), err
}

func (s *AuthService) checkPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

func (s *AuthService) issueToken(user *models.User) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
}

func (s *AuthService) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func (s *AuthService) Login(email, password string) (*models.User, string, error) {
	user, err := s.users.FindByEmail(email)
	if err != nil {
		return nil, "", ErrInvalidCredentials
	}
	if !s.checkPassword(user.Password, password) {
		return nil, "", ErrInvalidCredentials
	}
	token, err := s.issueToken(user)
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}

func (s *AuthService) Register(email, password, name string) (*models.User, string, error) {
	hash, err := s.HashPassword(password)
	if err != nil {
		return nil, "", err
	}
	user := &models.User{ID: uuid.New(), Email: email, Password: hash, Name: name}
	if err := s.users.Create(user); err != nil {
		return nil, "", err
	}
	token, err := s.issueToken(user)
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}

func (s *AuthService) GetUser(id uuid.UUID) (*models.User, error) {
	return s.users.FindByID(id)
}

func (s *AuthService) UpdateUser(id uuid.UUID, name string) (*models.User, error) {
	user, err := s.users.FindByID(id)
	if err != nil {
		return nil, err
	}
	user.Name = name
	if err := s.users.Update(user); err != nil {
		return nil, err
	}
	return user, nil
}
