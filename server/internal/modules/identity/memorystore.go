package identity

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// MemoryStore keeps local development and HTTP smoke tests usable without MySQL.
type MemoryStore struct {
	mu     sync.RWMutex
	nextID uint64
	users  []User
}

func NewMemoryStore() *MemoryStore {
	hash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	now := time.Now().UTC()
	return &MemoryStore{
		nextID: 2,
		users:  []User{{ID: 1, Username: "admin", PasswordHash: string(hash), Role: UserRoleAdmin, Nickname: "管理员", Status: UserStatusActive, CreatedAt: now, UpdatedAt: now}},
	}
}

func (s *MemoryStore) CreateUser(_ context.Context, params CreateUserParams) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.users {
		if strings.EqualFold(item.Username, strings.TrimSpace(params.Username)) {
			return User{}, ErrUsernameTaken
		}
	}
	now := time.Now().UTC()
	item := User{ID: s.nextID, Username: strings.TrimSpace(params.Username), PasswordHash: params.PasswordHash, Role: params.Role, Nickname: strings.TrimSpace(params.Nickname), Avatar: strings.TrimSpace(params.Avatar), Status: params.Status, CreatedAt: now, UpdatedAt: now}
	s.nextID++
	s.users = append(s.users, item)
	return item, nil
}

func (s *MemoryStore) FindUserByID(_ context.Context, id uint64) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.users {
		if item.ID == id {
			return item, nil
		}
	}
	return User{}, ErrUserNotFound
}

func (s *MemoryStore) FindUserByUsername(_ context.Context, username string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	username = strings.TrimSpace(username)
	for _, item := range s.users {
		if strings.EqualFold(item.Username, username) {
			return item, nil
		}
	}
	return User{}, ErrUserNotFound
}

func (s *MemoryStore) ListUsers(_ context.Context) ([]User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	users := slices.Clone(s.users)
	for i := range users {
		users[i].PasswordHash = ""
	}
	return users, nil
}

func (s *MemoryStore) UpdateUser(_ context.Context, params UpdateUserParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.users {
		if s.users[index].ID != params.ID {
			continue
		}
		if params.PasswordHash != "" {
			s.users[index].PasswordHash = params.PasswordHash
		}
		s.users[index].Role = params.Role
		s.users[index].Nickname = strings.TrimSpace(params.Nickname)
		s.users[index].Avatar = strings.TrimSpace(params.Avatar)
		s.users[index].Status = params.Status
		s.users[index].UpdatedAt = time.Now().UTC()
		return nil
	}
	return fmt.Errorf("%w: user %d", ErrUserNotFound, params.ID)
}

func (s *MemoryStore) UpdateProfile(_ context.Context, params UpdateProfileParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.users {
		if s.users[index].ID == params.ID {
			s.users[index].Nickname = strings.TrimSpace(params.Nickname)
			s.users[index].Avatar = strings.TrimSpace(params.Avatar)
			s.users[index].UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return ErrUserNotFound
}

func (s *MemoryStore) SetUserStatus(_ context.Context, params SetUserStatusParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.users {
		if s.users[index].ID == params.ID {
			s.users[index].Status = params.Status
			s.users[index].UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return ErrUserNotFound
}
